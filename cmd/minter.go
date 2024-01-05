package cmd

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gorilla/websocket"
	"github.com/nbd-wtf/go-nostr"
	"github.com/parnurzeal/gorequest"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

func init() {
	RootCmd.PersistentFlags().StringVar(&proxyUserName, "proxy-user", "1060398933010173952", "Proxy user name")
	RootCmd.PersistentFlags().StringVar(&proxyPassword, "proxy-password", "0w85A4wm", "Proxy password")
	RootCmd.PersistentFlags().IntVar(&numberOfWorkers, "workers", 10, "Number of workers")
}

var (
	proxyUserName   string
	proxyPassword   string
	numberOfWorkers int

	MintCmd = &cobra.Command{
		Use: "mint",
		Run: func(cmd *cobra.Command, args []string) {
			if proxyUserName == "" || proxyPassword == "" {
				cmd.Println("Please input proxy user name and password")
				return
			}

			walletData, err := os.ReadFile("wallet.csv")
			if err != nil {
				cmd.Println(err)
				return
			}

			csvReader := csv.NewReader(bytes.NewBuffer(walletData))

			records, err := csvReader.ReadAll()
			if err != nil {
				cmd.Println(err)
				return
			}

			mint(records[1:])
			return
		},
	}
)

var (
	blockNumber uint64
	messageId   string
	hash        string

	arbRpcUrl      = "https://rpc.ankr.com/arbitrum"
	currentWorkers int32
)

var (
	ErrDifficultyTooLow = errors.New("nip13: insufficient difficulty")
	ErrGenerateTimeout  = errors.New("nip13: generating proof of work took too long")
)

type Message struct {
	EventId string `json:"eventId"`
}

type EV struct {
	Sig       string          `json:"sig"`
	Id        string          `json:"id"`
	Kind      int             `json:"kind"`
	CreatedAt nostr.Timestamp `json:"created_at"`
	Tags      nostr.Tags      `json:"tags"`
	Content   string          `json:"content"`
	PubKey    string          `json:"pubkey"`
}

func mine(ctx context.Context, pubKey, privateKey string) {

	replayUrl := "wss://relay.noscription.org/"
	difficulty := 21

	// Create a channel to signal the finding of a valid nonce
	foundEvent := make(chan nostr.Event, 1)
	defer func() {
		for range foundEvent {
		}

		close(foundEvent)
	}()

	// Create a channel to signal all workers to stop

	ev := nostr.Event{
		Content:   `{"p":"nrc-20","op":"mint","tick":"noss","amt":"10"}`,
		CreatedAt: nostr.Now(),
		ID:        "",
		Kind:      nostr.KindTextNote,
		PubKey:    pubKey,
		Sig:       "",
		Tags:      nil,
	}
	ev.Tags = ev.Tags.AppendUnique(nostr.Tag{"p", "9be107b0d7218c67b4954ee3e6bd9e4dba06ef937a93f684e42f730a0c3d053c"})
	ev.Tags = ev.Tags.AppendUnique(nostr.Tag{"e", "51ed7939a984edee863bfbb2e66fdc80436b000a8ddca442d83e6a2bf1636a95", replayUrl, "root"})

	// Start multiple worker goroutines
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:

				if blockNumber == 0 {
					continue
				}

				evCopy := ev
				evCopy.Tags = ev.Tags.AppendUnique(nostr.Tag{"e", messageId, replayUrl, "reply"})
				evCopy.Tags = ev.Tags.AppendUnique(nostr.Tag{"seq_witness", strconv.Itoa(int(blockNumber)), hash})

				evCopy, err := Generate(evCopy, difficulty)
				if err != nil {
					// generate cost too long time
					//fmt.Println(err)
					continue
				}

				foundEvent <- evCopy
			}
		}
	}()

	select {
	case <-ctx.Done():
		fmt.Print("done")

	case evNew := <-foundEvent:
		startTime := time.Now()

		if err := evNew.Sign(privateKey); err != nil {
			fmt.Println("err: ", err)
		}

		evNewInstance := EV{
			Sig:       evNew.Sig,
			Id:        evNew.ID,
			Kind:      evNew.Kind,
			CreatedAt: evNew.CreatedAt,
			Tags:      evNew.Tags,
			Content:   evNew.Content,
			PubKey:    evNew.PubKey,
		}
		// 将ev转为Json格式
		eventJSON, err := json.Marshal(evNewInstance)
		if err != nil {
			log.Fatal(err)
		}

		wrapper := map[string]json.RawMessage{
			"event": eventJSON,
		}

		// 将包装后的对象序列化成JSON
		wrapperJSON, err := json.MarshalIndent(wrapper, "", "  ") // 使用MarshalIndent美化输出
		if err != nil {
			log.Fatalf("Error marshaling wrapper: %v", err)
		}

		request := gorequest.New().
			Proxy(fmt.Sprintf("http://%s:%s@http-dynamic.xiaoxiangdaili.com:10030", proxyUserName, proxyPassword))

		resp, _, errs := request.
			AppendHeader("Content-Type", "application/json").
			Post("https://api-worker.noscription.org/inscribe/postEvent").
			Send(bytes.NewBuffer(wrapperJSON)).
			End()

		if errs != nil {
			fmt.Printf("出现异常：%s\n", errs)
			return
		}

		spendTime := time.Since(startTime)
		// fmt.Println("Response Body:", string(body))

		if resp.StatusCode != 200 {
			fmt.Println("err: ", resp.StatusCode)
			return
		}

		fmt.Printf("%s spend: %s %d published to: %s\n", nostr.Now().Time(), spendTime, resp.StatusCode, evNew.ID)
	}

}

func connectToWSS(url string) (*websocket.Conn, error) {
	var conn *websocket.Conn
	var err error
	headers := http.Header{}
	headers.Add("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0")
	headers.Add("Origin", "https://noscription.org")
	headers.Add("Host", "report-worker-2.noscription.org")
	for {
		// 使用gorilla/websocket库建立连接
		conn, _, err = websocket.DefaultDialer.Dial(url, headers)
		fmt.Println("Connecting to wss")
		if err != nil {
			// 连接失败，打印错误并等待一段时间后重试
			fmt.Println("Error connecting to WebSocket:", err)
			// time.Sleep(1 * time.Second) // 5秒重试间隔
			continue
		}
		// 连接成功，退出循环
		break
	}
	return conn, nil
}

func mint(record [][]string) {

	ctx := context.Background()
	wssAddr := "wss://report-worker-2.noscription.org"
	// relayUrl := "wss://relay.noscription.org/"

	var err error

	client, err := ethclient.Dial(arbRpcUrl)
	if err != nil {
		log.Fatalf("无法连接到Arbitrum节点: %v", err)
	}

	c, err := connectToWSS(wssAddr)
	if err != nil {
		panic(any(err))
	}
	defer c.Close()

	// initialize an empty cancel function

	// get block
	go func() {
		for {
			header, err := client.HeaderByNumber(context.Background(), nil)
			if err != nil {
				log.Fatalf("无法获取最新区块号: %v", err)
			}
			if header.Number.Uint64() >= blockNumber {
				hash = header.Hash().Hex()
				blockNumber = header.Number.Uint64()
			}
		}
	}()

	go func() {
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				break
			}

			var messageDecode Message
			if err := json.Unmarshal(message, &messageDecode); err != nil {
				fmt.Println(err)
				continue
			}
			messageId = messageDecode.EventId
		}

	}()

	atomic.StoreInt32(&currentWorkers, 0)
	// 初始化一个取消上下文和它的取消函数
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 监听blockNumber和messageId变化
	go func() {
		for {
			select {
			case <-ctx.Done(): // 如果上下文被取消，则退出协程
				return
			default:
				if atomic.LoadInt32(&currentWorkers) < int32(numberOfWorkers) {
					atomic.AddInt32(&currentWorkers, 1) // 增加工作者数量
					fmt.Println("init worker ", currentWorkers)

					go func() {
						defer atomic.AddInt32(&currentWorkers, -1) // 完成后减少工作者数量
						wg := &sync.WaitGroup{}

						for _, row := range record {
							wg.Add(1)

							pubKey := row[2]
							privateKey := row[3]

							go func() {
								defer wg.Done()
								mine(ctx, pubKey, privateKey)
							}()
						}

						wg.Wait()
					}()
				}
			}
		}
	}()

	select {}

}

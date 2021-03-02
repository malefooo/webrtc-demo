package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"time"
)

/*
	offer和answer是一样的，都作为一个点
	可以当offer，也可以做answer

	流程如下：
	peer ---register---> server ---id---> peer
	peer(offer) ---offerReq---> server ---offerReq---> peer(answer) ---answerResp---> server ---answerResp---> peer(offer)


*/

//服务器给过来的消息体，分为offer和answer
type OfferReq struct {
	OfferSdp     string   `json:"offer_sdp"`
	Candidate    []string `json:"candidate"`
	AnswerDomain string   `json:"answer_domain"`
	OfferDomain  string   `json:"offer_domain"`
}

type AnswerResp struct {
	AnswerSdp   []byte   `json:"answer_sdp"`
	OfferDomain string   `json:"offer_domain"`
	Candidate   [][]byte `json:"candidate"`
}

type Obj struct {
	Ty         string      `json:"ty"` //"answer","offer"
	AnswerResp *AnswerResp `json:"answer_resp"`
	OfferReq   *OfferReq   `json:"offer_req"`
}


func main() {
	//var id string
	//var candidate []byte
	pendingCandidates := make([][]byte, 0)
	pendingCandidates2 := make([]string, 0)
	var candidatesMux sync.Mutex
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
			{
				URLs: []string{"turn:192.158.29.39?transport=udp"},
				Username: "unittest",
				Credential: "placeholder",
				CredentialType: webrtc.ICECredentialTypePassword,
			},
		},
	}

	peerConnection, err := webrtc.NewPeerConnection(config)
	if err != nil {
		panic(err)
	}

	peerConnection.OnICECandidate(func(c *webrtc.ICECandidate) {
		fmt.Println("candidate:",c,time.Now().Unix() )
		if c == nil {
			return
		}
		candidatesMux.Lock()
		defer candidatesMux.Unlock()

		pendingCandidates2 = append(pendingCandidates2, c.ToJSON().Candidate)
		pendingCandidates = append(pendingCandidates, []byte(c.ToJSON().Candidate))
	})

	//建立websocket链接
	u := url.URL{
		Scheme:      "ws",
		Host: "127.0.0.1:9091",
		Path:        "/ws/A",
	}

	c,resp,_ := websocket.DefaultDialer.Dial(u.String(),nil)
	fmt.Println(resp)
	//接受连通后第一次回调给过来的id，后期可以改为Text类型来接收
	//c.SetPongHandler(func(appData string) error {
	//	if appData != "" {
	//		id = appData
	//	}
	//	return nil
	//})

	defer c.Close()

	//设置接受服务器消息
	go func() {
		for  {
			ty, message, _ := c.ReadMessage()
			if ty < 0 {
				return
			}
			fmt.Printf("recv: %s\n", message)

			obj := Obj{}
			_ = json.Unmarshal(message, &obj)

			switch obj.Ty {
			case "offer":
				//收到远程offer，创建answer
				offerReq := obj.OfferReq
				sdp := webrtc.SessionDescription{}
				offerSdp_bytes,err := base64.StdEncoding.DecodeString(offerReq.OfferSdp)
				if err!=nil {
					fmt.Println("err:",err)
				}

				//设置远程SDP
				err = json.Unmarshal(offerSdp_bytes, &sdp)
				_ = peerConnection.SetRemoteDescription(sdp)

				//添加远程peer的candidate，即ip地址啥的
				for _,c := range offerReq.Candidate {
					fmt.Println("c:",string(c),time.Now().Unix())
					err = peerConnection.AddICECandidate(webrtc.ICECandidateInit{Candidate: string(c)})
					if err != nil {
						fmt.Println("add candidate:",err)
					}
				}
				//设置连接状态改变监听
				peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
					fmt.Printf("ICE Connection State has changed: %s\n", connectionState.String())
				})

				//设置通道，和offer的通道不一样
				//offer是创建通道，answer是配置通道吧
				peerConnection.OnDataChannel(func(d *webrtc.DataChannel) {
					fmt.Printf("New DataChannel %s %d\n", d.Label(), d.ID())

					// Register channel opening handling
					d.OnOpen(func() {
						fmt.Printf("Data channel '%s'-'%d' open. Random messages will now be sent to any connected DataChannels every 5 seconds\n", d.Label(), d.ID())

						for range time.NewTicker(5 * time.Second).C {
							message := rand.Int()
							fmt.Printf("Sending '%d'\n", message)

							// Send the message as text
							sendTextErr := d.SendText(strconv.Itoa(message))
							if sendTextErr != nil {
								panic(sendTextErr)
							}
						}
					})

					// Register text message handling
					d.OnMessage(func(msg webrtc.DataChannelMessage) {
						fmt.Printf("Message from DataChannel '%s': '%s'\n", d.Label(), string(msg.Data))
					})
				})


				answer,err := peerConnection.CreateAnswer(nil)
				if err != nil {
					fmt.Println("CreateAnswer:",err)
					return
				}

				err = peerConnection.SetLocalDescription(answer)
				if err != nil {
					fmt.Println("SetLocalDescription:",err)
					return
				}
				answerSdp, err := json.Marshal(answer)
				if err != nil {
					fmt.Println("answer Marshal:",err)
					return
				}

				answerResp := AnswerResp{
					AnswerSdp: answerSdp,
					OfferDomain:   offerReq.OfferDomain,
				}

				obj.Ty = "answer"
				obj.AnswerResp = &answerResp

				jsons,err := json.Marshal(obj)
				if err != nil {
					fmt.Println("obj Marshal:",err)
					return
				}
				err = c.WriteMessage(websocket.TextMessage, jsons)
				if err != nil {
					fmt.Println("WriteMessage:",err)
					return
				}

			case "answer":
				//接收来自answer的消息
				answer := obj.AnswerResp
				sdp := webrtc.SessionDescription{}
				err := json.Unmarshal(answer.AnswerSdp, &sdp)
				if err !=nil{
					fmt.Println("answer Unmarshal:",err)
					return
				}

				//设置SDP
				err = peerConnection.SetRemoteDescription(sdp)
				if err != nil {
					fmt.Println("SetRemoteDescription:",err)
				}

				for _, c := range answer.Candidate {
					fmt.Println("c:", string(c), time.Now().Unix())
					err = peerConnection.AddICECandidate(webrtc.ICECandidateInit{Candidate: string(c)})
					if err != nil {
						fmt.Println("add candidate:", err)
					}
				}

			}

		}
	}()


	//for id == ""{
	//	time.Sleep(3 * time.Second)
	//}
	//发出offer
	var offer = func() {

		//创建通道
		dataChannel, err := peerConnection.CreateDataChannel("data", nil)
		if err != nil {
			fmt.Println("CreateDataChannel:",err)
		}

		//ICE连接状态改变监听
		peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
			fmt.Printf("ICE Connection State has changed: %s\n", connectionState.String())
		})

		//打开通道
		dataChannel.OnOpen(func() {
			fmt.Printf("Data channel '%s'-'%d' open. Random messages will now be sent to any connected DataChannels every 5 seconds\n", dataChannel.Label(), dataChannel.ID())

			for range time.NewTicker(5 * time.Second).C {
				//message := signal.RandSeq(15)
				message := rand.Int()
				fmt.Printf("Sending '%d'\n", message)

				// Send the message as text
				sendTextErr := dataChannel.SendText(strconv.Itoa(message))
				if sendTextErr != nil {
					panic(sendTextErr)
				}
			}
		})

		dataChannel.OnClose(func() {
			fmt.Println("data channel close")
		})

		//接受消息
		dataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
			fmt.Printf("Message from DataChannel '%s': '%s'\n", dataChannel.Label(), string(msg.Data))
		})

		//创建offerSDP
		offer, _ := peerConnection.CreateOffer(nil)

		//设置本地SDP
		_ = peerConnection.SetLocalDescription(offer)


		offerSdp,_ := json.Marshal(offer)
		//等待500毫秒，要不candidate收不到，这个实在offer设置完本地SDP后触发回调
		time.Sleep(500 * time.Millisecond)
		offer_base64 := base64.StdEncoding.EncodeToString(offerSdp)

		req := OfferReq{
			OfferSdp: offer_base64,
			AnswerDomain: "B",
			OfferDomain:  "A",
			Candidate: pendingCandidates2,
		}
		fmt.Println(req)
		s,err := json.Marshal(&req)
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println(s)
		//发给服务器
		resp, err := http.Post("http://127.0.0.1:9091/offer", "application/json;", bytes.NewReader(s))
		if err != nil {
			fmt.Println(err)
		}
		fmt.Println(resp)
	}

	offer()

	s := make(chan os.Signal)
	//监听指定信号 ctrl+c kill
	signal.Notify(s, os.Interrupt, os.Kill)
	//阻塞直到有信号传入
	fmt.Println("启动")
	//阻塞直至有信号传入
	select {
	case <-s:
		_ = peerConnection.Close()
	}

	//心跳
	//ticker := time.NewTicker(5 * time.Second)
	//for  {
	//	select {
	//	case <-ticker.C:
	//		if err := c.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
	//			fmt.Println("ping:", err)
	//		}
	//	}
	//}
}

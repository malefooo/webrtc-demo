package swebrtc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/gorilla/websocket"
	"github.com/pion/webrtc"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"time"
	"webrtc-test/answer-go/config"
	"webrtc-test/answer-go/proto/warpQP"
	"webrtc-test/answer-go/server/sgrpc"
)

type Obj struct {
	Ty         string      `json:"ty"` //"answer","offer"
	AnswerResp *AnswerResp `json:"answer_resp"`
	OfferReq   *OfferReq   `json:"offer_req"`
}

type OfferReq struct {
	OfferSdp     string   `json:"offer_sdp"`
	Candidate    []string `json:"candidate"`
	AnswerDomain string   `json:"answer_domain"`
	OfferDomain  string   `json:"offer_domain"`
}

type AnswerResp struct {
	AnswerSdp   string   `json:"answer_sdp"`
	OfferDomain string   `json:"offer_domain"`
	Candidate   []string `json:"candidate"`
}


type SWebRtc struct {
	g *config.Global
	grpcClient *sgrpc.GrpcClient
}

func NewAnswer(g *config.Global, grpcClient *sgrpc.GrpcClient) *SWebRtc{

	server := SWebRtc{g: g, grpcClient: grpcClient}

	return &server
}

func (s *SWebRtc)New()  {
	s.g.Log.Infoln("server start")

	pendingCandidates := make([]string, 0)
	var candidatesMux sync.Mutex
	//建立websocket链接
	u := url.URL{
		Scheme: "ws",
		Host:   s.g.Config.Server.SignalServerHost,
		Path:   s.g.Config.Server.Path + "/" + s.g.Config.Server.Domain,
	}

	c, resp, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		s.g.Log.Errorln(err)
		return
	}
	s.g.Log.Infoln(resp)

	iceServices := make([]webrtc.ICEServer, 0)

	//填充stun
	for _, v := range s.g.Config.IceServers.Stun {
		iceServices = append(iceServices, webrtc.ICEServer{
			URLs: []string{v.Urls},
		})
	}

	//填充turn
	for _, v := range s.g.Config.IceServers.Turn {

		flag := webrtc.ICECredentialTypePassword
		if v.CredentialType == 1 {
			flag = webrtc.ICECredentialTypeOauth
		}

		iceServices = append(iceServices, webrtc.ICEServer{
			URLs:           []string{v.Urls},
			Username:       v.UserName,
			Credential:     v.Credential,
			CredentialType: flag,
		})
	}

	defer c.Close()

	//设置接受消息
	go func() {
		for {
			ty, message, err := c.ReadMessage()
			if err != nil {
				s.g.Log.Errorln(err)
				break
			}
			if ty < 0 {
				return
			}
			s.g.Log.Infoln(message)

			obj := Obj{}
			err = json.Unmarshal(message, &obj)
			if err != nil {
				s.g.Log.Infoln(err)
				continue
			}

			//判断消息类型
			switch obj.Ty {
			case "offer":
				//收到远程offer，创建answer
				offerReq := obj.OfferReq
				sdp := webrtc.SessionDescription{}
				offerSdp_bytes,err := base64.StdEncoding.DecodeString(offerReq.OfferSdp)
				if err != nil {
					s.g.Log.Errorln(err)
					return
				}
				err = json.Unmarshal(offerSdp_bytes, &sdp)
				if err != nil {
					s.g.Log.Errorln(err)
					return
				}

				config := webrtc.Configuration{
					ICEServers: iceServices,
				}

				peerConnection, err := webrtc.NewPeerConnection(config)
				if err != nil {
					panic(err)
				}

				peerConnection.OnICECandidate(func(c *webrtc.ICECandidate) {
					if c == nil {
						return
					}
					candidatesMux.Lock()
					defer candidatesMux.Unlock()

					pendingCandidates = append(pendingCandidates, c.ToJSON().Candidate)
				})

				err = peerConnection.SetRemoteDescription(sdp)
				if err != nil {
					panic(err)
				}

				for _, c := range offerReq.Candidate {
					err = peerConnection.AddICECandidate(webrtc.ICECandidateInit{Candidate: string(c)})
					if err != nil {
						s.g.Log.Errorln(err)
					}
				}

				peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
					fmt.Printf("ICE Connection State has changed: %s\n", connectionState.String())
					if connectionState.String() == "disconnected" {
						err = peerConnection.Close()
						if err != nil {
							panic(err)
						}
					}
				})

				// Register data channel creation handling
				peerConnection.OnDataChannel(func(d *webrtc.DataChannel) {
					fmt.Printf("New DataChannel %s %d\n", d.Label(), d.ID())

					// Register channel opening handling
					d.OnOpen(func() {
						//fmt.Printf("Data channel '%s'-'%d' open. Random messages will now be sent to any connected DataChannels every 5 seconds\n", d.Label(), d.ID())

						//for range time.NewTicker(5 * time.Second).C {
						//
						//	message := rand.Int()
						//	fmt.Printf("Sending '%d'\n", message)
						//
						//	// Send the message as text
						//	sendTextErr := d.SendText(strconv.Itoa(message))
						//	if sendTextErr != nil {
						//		fmt.Println(sendTextErr)
						//		break
						//	}
						//}
					})

					d.OnClose(func() {
						s.g.Log.Infoln("channel close")
					})

					// Register text message handling
					d.OnMessage(func(msg webrtc.DataChannelMessage) {
						s.g.Log.Infoln("Message from DataChannel '%s': '%s'\n", d.Label(), string(msg.Data))

						resp,err := s.messageHandle(msg.Data)
						if err != nil {
							s.g.Log.Errorln(err)
							return
						}

						b,err := proto.Marshal(resp)
						if err != nil {
							s.g.Log.Errorln(err)
							return
						}

						err = d.Send(b)
						if err != nil {
							s.g.Log.Errorln(err)
							return
						}
					})
				})

				answer, err := peerConnection.CreateAnswer(nil)
				if err != nil {
					s.g.Log.Errorln(err)
					return
				}
				err = peerConnection.SetLocalDescription(answer)
				if err != nil {
					s.g.Log.Errorln(err)
					return
				}
				//让stun或者turn返回ice地址并且填充
				time.Sleep(500 * time.Millisecond)

				answerSdp, err := json.Marshal(answer)
				if err != nil {
					s.g.Log.Errorln()
					return
				}

				answerSdp_str := base64.StdEncoding.EncodeToString(answerSdp)

				answerResp := AnswerResp{
					AnswerSdp:   answerSdp_str,
					OfferDomain: offerReq.OfferDomain,
					Candidate:   pendingCandidates,
				}

				obj.Ty = "answer"
				obj.AnswerResp = &answerResp

				jsons, err := json.Marshal(obj)
				if err != nil {
					s.g.Log.Errorln(err)
					return
				}
				err = c.WriteMessage(websocket.TextMessage, jsons)
				if err != nil {
					s.g.Log.Errorln(err)
					return
				}
			}
		}
	}()

	ch := make(chan os.Signal)
	//监听指定信号 ctrl+c kill
	signal.Notify(ch, os.Interrupt, os.Kill)
	//阻塞直到有信号传入
	fmt.Println("启动")
	//阻塞直至有信号传入
	select {
	case <-ch:
		fmt.Println("over")
	}

	//s := make(chan os.Signal)
	////监听指定信号 ctrl+c kill
	//signal.Notify(s, os.Interrupt, os.Kill, syscall.SIGUSR1, syscall.SIGUSR2)
	//
	////设置心跳，10s一次
	//ticker := time.NewTicker(5 * time.Second)
	//for {
	//	select {
	//	case <-ticker.C:
	//		if err := c.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
	//			panic(err)
	//		}
	//	case <-s:
	//		Log.Infoln("server over")
	//	}
	//}
}

func (s *SWebRtc)messageHandle(message []byte)  (*warpQP.WarpResp,error){

	wq := warpQP.WarpReq{}
	wp := warpQP.WarpResp{}
	err := proto.Unmarshal(message, &wq)
	if err != nil {
		s.g.Log.Errorln(err)
	}

	switch wq.GetType() {
	case warpQP.ReqType_LoginReq:
		resp,err := s.grpcClient.DeviceClient.AuthLogin(context.Background(), wq.GetLoginReq())
		if err != nil {
			return nil,err
		}
		wp.Type = warpQP.RespType_ResultReplyLogin
		wp.RespData = &warpQP.WarpResp_ResultReplyLogin{ResultReplyLogin: resp}
	case warpQP.ReqType_ComputeReq:
		resp,err := s.grpcClient.DeviceClient.ComputeS(context.Background(), wq.GetComputeReq())
		if err != nil {
			return nil,err
		}
		wp.Type = warpQP.RespType_ResultReplyCompute
		wp.RespData = &warpQP.WarpResp_ResultReplyCompute{ResultReplyCompute: resp}
	case warpQP.ReqType_PasswdReq:
		resp,err := s.grpcClient.DeviceClient.Passwd(context.Background(), wq.GetPasswdReq())
		if err != nil {
			return nil,err
		}
		wp.Type = warpQP.RespType_ResultReplyPasswd
		wp.RespData = &warpQP.WarpResp_ResultReplyPasswd{ResultReplyPasswd: resp}
	case warpQP.ReqType_OSSInfoSubmit:
		resp,err := s.grpcClient.DeviceClient.OSSInfoSubmit(context.Background(), wq.GetOSSTrueReq())
		if err != nil {
			return nil,err
		}
		wp.Type = warpQP.RespType_ResultReplyOSS
		wp.RespData = &warpQP.WarpResp_ResultReplyOSS{ResultReplyOSS: resp}
	case warpQP.ReqType_OSSInfoCheck:
		resp,err := s.grpcClient.DeviceClient.OSSInfoCheck(context.Background(), wq.GetOSSInfo())
		if err != nil {
			return nil,err
		}
		wp.Type = warpQP.RespType_ResultReplyOSS
		wp.RespData = &warpQP.WarpResp_ResultReplyOSS{ResultReplyOSS: resp}
	}

	return &wp,nil
}

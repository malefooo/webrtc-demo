

package main

import (
"encoding/json"
"flag"
"fmt"
"github.com/BurntSushi/toml"
"github.com/gorilla/mux"
"github.com/gorilla/websocket"
"io"
"net/http"
"github.com/sirupsen/logrus"
"os"
"path/filepath"
)


var KV_MAP = make(map[string]*websocket.Conn)
var VK_MAP = make(map[*websocket.Conn]string)

type Config struct {
	Server struct {
		Host string `toml:"host"`
	} `toml:"server"`
	Log struct {
		OutDir  string `toml:"out_dir"`
		OutFile string `toml:"out_file"`
	} `toml:"log"`
}

type OfferReq struct {
	OfferSdp	string
	Candidate []string
	AnswerId	string
	OfferId		string
}

type AnswerResp struct {
	AnswerSdp    string
	OfferId        string
	Candidate []string
}

type Obj struct {
	Ty string //"answer","offer"
	AnswerResp *AnswerResp
	OfferReq *OfferReq
}

type Resp struct {
	Code int `json:"code"`
	Msg string `json:"msg"`
	Data interface{} `json:"data"`
}

var Log = logrus.New()


func main() {
	//server地址
	config := parse_config_toml()
	init_log(config)

	router := mux.NewRouter().StrictSlash(true)
	flag.Parse()
	router.HandleFunc("/ws/{domain}", ws)
	router.HandleFunc("/offer",offer).Methods("POST")
	fmt.Println("server start")
	err := http.ListenAndServe(config.Server.Host, router)
	if err != nil {
		fmt.Println(err)
	}
}

func ws(w http.ResponseWriter, r *http.Request){
	//w.WriteHeader(200)
	var upgrader = websocket.Upgrader{}
	params := mux.Vars(r)
	domain := params["domain"]

	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		Log.Errorln(err)
		json.NewEncoder(w).Encode(Resp{
			Code: -1,
			Msg:  err.Error(),
			Data: nil,
		})
		return
	}

	KV_MAP[domain] = c
	VK_MAP[c] = domain

	fmt.Println(KV_MAP)
	fmt.Println(VK_MAP)
	defer c.Close()
	for {
		//接受消息
		mt, message, err := c.ReadMessage()
		key := VK_MAP[c]
		//定制log
		log := Log.WithFields(map[string]interface{}{
			"domain":key,
		})
		if err != nil {
			log.Errorln(err)
			delete(VK_MAP, c)
			delete(KV_MAP,key)
			fmt.Println("delete:",KV_MAP)
			fmt.Println("delete:",VK_MAP)
			break
		}
		//如果是ping直接返回pong，心跳
		if mt == websocket.PingMessage {
			err = c.WriteMessage(websocket.PongMessage, message)
			if err != nil {
				log.Errorln(err)
				continue
			}
		}

		log.Infoln(message)

		obj := Obj{}
		err = json.Unmarshal(message, &obj)
		if err != nil{
			log.Errorln(err)
			continue
		}
		obj.OfferReq = nil

		//获取offer的ws链接
		offerC := KV_MAP[obj.AnswerResp.OfferId]
		jsons,err := json.Marshal(&obj)
		if err != nil {
			log.Errorln(err)
			continue
		}
		err = offerC.WriteMessage(websocket.TextMessage, jsons)
		if err != nil {
			log.Errorln(err)
			continue
		}
	}
}

//offer请求
func offer(w http.ResponseWriter, r *http.Request){
	w.WriteHeader(200)
	decoder := json.NewDecoder(r.Body)
	req := OfferReq{}


	err := decoder.Decode(&req)
	if err != nil {
		Log.Errorln(err)
		json.NewEncoder(w).Encode(Resp{
			Code: -1,
			Msg:  err.Error(),
			Data: nil,
		})
		return
	}
	Log.Infoln(req)

	obj := Obj{
		Ty:   "offer",
		OfferReq: &req,
	}
	s, err := json.Marshal(&obj)
	if err != nil {
		Log.Errorln(err)
		json.NewEncoder(w).Encode(Resp{
			Code: -1,
			Msg:  err.Error(),
			Data: nil,
		})
		return
	}

	//推送给answer
	c := KV_MAP[req.AnswerId]
	err = c.WriteMessage(websocket.TextMessage, s)
	if err != nil {
		Log.Errorln(err)
		return
	}

	json.NewEncoder(w).Encode(Resp{
		Code: 0,
		Msg:  "success",
		Data: nil,
	})
	return
}

//初始化日志
func init_log(c *Config){

	Log.Out = os.Stdout
	Log.Formatter = &logrus.JSONFormatter{}

	exist, err := pathExists(c.Log.OutDir)

	if err != nil {
		panic(err)
	}

	if !exist {
		Log.Printf("no dir![%v]\n", c.Log.OutDir)
		// 创建文件夹
		err := os.MkdirAll(c.Log.OutDir, os.ModePerm)
		if err != nil {
			Log.Printf("mkdir failed![%v]\n", err)
		} else {
			Log.Printf("mkdir success!\n")
		}
	}

	out_dir_file := c.Log.OutDir + "/" + c.Log.OutFile
	file, err := os.OpenFile(out_dir_file, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}

	writers := []io.Writer{file, os.Stdout}

	writer := io.MultiWriter(writers...)

	Log.SetReportCaller(true)
	Log.SetOutput(writer)
	Log.SetLevel(logrus.InfoLevel)
}

//解析toml文件
func parse_config_toml() *Config{
	var config Config
	filename, err := filepath.Abs("config.toml")
	if  err != nil{
		panic(err)
	}
	if _, err := toml.DecodeFile(filename, &config); err != nil {
		panic(err)
	}

	return &config
}

func pathExists(path string)(bool, error)  {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
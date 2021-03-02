package config

import (
	"github.com/BurntSushi/toml"
	"github.com/sirupsen/logrus"
	"io"
	"os"
	"path/filepath"
	"webrtc-test/answer-go/utils"
)

//var G = new(global)

//全局变量
type Global struct {
	Log *logrus.Logger
	Config *Config
}

type Config struct {
	Server struct {
		SignalServerHost string `toml:"signal_server_host"`
		Path             string `toml:"path"`
		Domain           string `toml:"domain"`
		DeviceHost       string `toml:"device_host"`
	} `toml:"server"`
	IceServers struct {
		Stun []struct {
			Urls string `toml:"urls"`
		} `toml:"stun"`
		Turn []struct {
			Urls           string `toml:"urls"`
			UserName       string `toml:"user_name"`
			Credential     string `toml:"credential"`
			CredentialType int    `toml:"credential_type"`
		} `toml:"turn"`
	} `toml:"ice_servers"`
	Log struct {
		OutDir  string `toml:"out_dir"`
		OutFile string `toml:"out_file"`
	} `toml:"log"`
}



func New() *Global{
	G := Global{}
	G.Log = logrus.New()
	G.parse_config_toml()
	G.init_log()

	return &G
}

func (g *Global)parse_config_toml() {
	var config Config
	filename, err := filepath.Abs("config.toml")
	if err != nil {
		panic(err)
	}
	if _, err := toml.DecodeFile(filename, &config); err != nil {
		panic(err)
	}

	if len(config.IceServers.Stun) == 0 {
		panic("the stun is nil")
	}

	if len(config.IceServers.Turn) == 0 {
		panic("the turn is nil")
	}
	g.Config = &config
}


//初始化日志
func (g *Global)init_log() {

	g.Log.Out = os.Stdout
	g.Log.Formatter = &logrus.JSONFormatter{}

	exist, err := utils.PathExists(g.Config.Log.OutDir)

	if err != nil {
		panic(err)
	}

	if !exist {
		g.Log.Printf("no dir![%v]\n", g.Config.Log.OutDir)
		// 创建文件夹
		err := os.MkdirAll(g.Config.Log.OutDir, os.ModePerm)
		if err != nil {
			g.Log.Printf("mkdir failed![%v]\n", err)
		} else {
			g.Log.Printf("mkdir success!\n")
		}
	}

	out_dir_file := g.Config.Log.OutDir + "/" + g.Config.Log.OutFile
	file, err := os.OpenFile(out_dir_file, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}

	writers := []io.Writer{file, os.Stdout}

	writer := io.MultiWriter(writers...)

	g.Log.SetReportCaller(true)
	g.Log.SetOutput(writer)
	g.Log.SetLevel(logrus.InfoLevel)
}
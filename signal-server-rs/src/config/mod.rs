
use log::LevelFilter;
use log4rs::append::file::FileAppender;
use log4rs::encode::pattern::PatternEncoder;
use log4rs::config::{Appender, Config, Root, Logger};
use log4rs::encode::EncoderConfig;
use log4rs::encode::json::{JsonEncoder, JsonEncoderConfig};
use log4rs::append::console::ConsoleAppender;
use std::fs::File;
use std::io::Read;
use serde::{Deserialize, Serialize};

#[derive(Deserialize,Debug,Clone)]
pub struct C{
    pub server:Option<Server>,
    pub log:Option<Log>,
}

#[derive(Deserialize,Debug,Clone)]
pub struct Server{
    pub host:String
}

#[derive(Deserialize,Debug,Clone)]
pub struct Log{
    pub out_dir:String,
    pub out_file:String,
}

impl C {
    /// 新建c
    pub fn new() -> C{
        let mut c = C{ server: None, log: None };
        c.init_log();
        c.parsing_conf_toml();
        c
    }

    /// 初始化配置文件
    fn init_log(&mut self) {
        log4rs::init_file("./log4rs.yaml", Default::default()).unwrap();
        info!("init finish")
    }

    /// 读取配置文件
    fn parsing_conf_toml(&mut self){
        let file_path = "./conf.toml";
        let mut file = match File::open(file_path){
            Ok(f)=>f,
            Err(e)=>panic!("no such file {} exception:{}", file_path, e)
        };
        let mut str_val = String::new();
        match file.read_to_string(&mut str_val) {
            Ok(_s)=>{},
            Err(e) => panic!("Error Reading file: {}", e)
        }

        // let c = toml::from_str(&str_val).unwrap();
        let c = toml::from_str::<C>(&str_val).unwrap_or_else(|e|panic!(e));
        *self = c
    }
}




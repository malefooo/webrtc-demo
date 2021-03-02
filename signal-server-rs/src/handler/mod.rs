use warp::{Reply, Error, Rejection};
use futures::{StreamExt, TryFutureExt,FutureExt};
use warp::filters::ws::Message;
use tokio_stream::wrappers::UnboundedReceiverStream;
use tokio::sync::mpsc;
use crate::{KV_MAP};
use std::convert::Infallible;
use warp::http::StatusCode;
use serde::{Deserialize, Serialize};
use std::sync::{Arc, RwLock};

#[derive(Deserialize,Serialize,Debug,Clone,Default)]
pub struct OfferReq{
    pub offer_sdp:String,
    pub candidate:Vec<String>,
    pub answer_domain:String,
    pub offer_domain:String,
}

#[derive(Deserialize,Serialize,Debug,Clone,Default)]
pub struct AnswerResp{
    pub answer_sdp:String,
    pub offer_domain:String,
    pub candidate:Vec<String>,
}

#[derive(Deserialize,Serialize,Debug,Clone,Default)]
pub struct Obj{
    pub ty:String,
    pub answer_resp:Option<AnswerResp>,
    pub offer_req:Option<OfferReq>,
}

#[derive(Deserialize,Serialize,Debug,Clone,Default)]
pub struct Resp{
    pub code:i32,
    pub msg:Option<String>,
    pub data:Option<String>,
}

/// 处理websocket链接的逻辑
pub async fn ws_handler(websocket: warp::ws::WebSocket,domain:String){
    let uid = uuid::Uuid::new_v4().to_string();
    info!("uid:{},domain:{}",uid.clone(),domain);
    //得到发送者和接收者
    let(sender, mut rcv) = websocket.split();

    //创建无限制通道，
    let (client_sender, client_rcv) = mpsc::unbounded_channel();

    //使用无限制接收器创建无限制接收流
    let rx = UnboundedReceiverStream::new(client_rcv);

    //似乎是用无限制通道接收流来绑定发送者
    tokio::task::spawn(rx.forward(sender).map(|result| {
        if let Err(e) = result {
            error!("err:{}",e)
        }
    }));

    //存入全局map
    let kv_map = KV_MAP.get().unwrap();
    kv_map.write().await.insert(domain.clone(), client_sender);

    //等待接收数据
    while let Some(result) = rcv.next().await {
        let msg = match result {
            Ok(msg) => msg,
            Err(e) => {
                error!("uid:{},err:{}",uid.clone(),e);
                break;
            }
        };

        if msg.is_ping() || msg.is_pong(){
            info!("uid:{},msg:{}",uid.clone(),"ping or pong msg");
            continue;
        }

        if msg.is_text() {
            let result = serde_json::from_slice::<Obj>(msg.as_bytes());

            if result.is_err() {
                error!("uid:{},err:{:?}",uid.clone(),result.err());
                return;
            }

            let obj = result.unwrap();

            info!("uid:{},recv:{:?}",uid.clone(),obj);

            let offer_domain = obj.answer_resp.clone().unwrap().offer_domain.clone();
            let kv_map = KV_MAP.get().unwrap();
            let lock = kv_map.read().await;
            let op = lock.get(offer_domain.as_str());

            if op.is_none() {
                warn!("uid:{},offer_domain:{},msg:{}",uid.clone(),offer_domain.as_str(),"offer sender is nil");
                return;
            }

            let offer_sender = op.unwrap();


            let result = serde_json::to_string(&obj);

            if result.is_err(){
                error!("uid:{},err:{:?},msg:{}",uid.clone(),result.err(),"obj 2 string err");
                return;
            }

            let obj_str = result.unwrap();

            let result = offer_sender.send(Ok(Message::text(obj_str)));

            if result.is_err() {
                error!("uid:{},err:{:?}:msg:{}",uid.clone(),result.err(),"send to offer err");
                return;
            }

            info!("uid:{},result:{:?}",uid.clone(),result.unwrap())
        }

    }
}

/// 处理offer请求的逻辑
pub async fn offer_handler(offer_req: OfferReq) -> Result<Box<dyn warp::Reply>, warp::Rejection>{
    let uid = &uuid::Uuid::new_v4().to_string();
    info!("uid:{},offer_req:{:?}",uid,offer_req);
    let answer_domain = offer_req.answer_domain.clone();

    let obj = Obj{
        ty: "offer".to_string(),
        answer_resp: Default::default(),
        offer_req: Some(offer_req)
    };

    let result = serde_json::to_string(&obj);
    if result.is_err() {
        error!("uid:{},err:{:?}",uid,result.as_ref().err());
        return Ok(Box::new(create_resp(-1,Some(format!("{:?}",result.as_ref().err())),None)));
    }

    let kv_map = KV_MAP.get().unwrap();
    let lock = kv_map.read().await;
    let op = lock.get(answer_domain.as_str());

    if op.is_none() {
        warn!("uid:{},msg:{}",uid,"answer sender is nil");
        return Ok(Box::new(create_resp(-1,Some("answer sender is nil".to_string()),None)));
    }
    let sender = op.unwrap();
    let result = sender.send(Ok(Message::text(result.unwrap())));
    if result.is_err() {
        println!("send to answer err");
        error!("uid:{},err:{:?}",uid,result.as_ref().err());
        return Ok(Box::new(create_resp(-1,Some(format!("{:?}",result.as_ref().err())),None)));
    }

    return Ok(Box::new(create_resp(-1,Some("success".to_string()),None)));
}

pub async fn handle_rejection(
    err: Rejection
) -> std::result::Result<impl Reply, Infallible> {
    let our_ids = vec![1, 3, 7, 13];

    println!("{:?}",err);

    Ok(warp::reply::with_status(warp::reply::json(&our_ids),StatusCode::OK))
}

fn create_resp(code:i32,msg:Option<String>,data:Option<String>) -> String{
    let resp = Resp{
        code,
        msg,
        data,
    };
    serde_json::to_string(&resp).unwrap_or_else(|e|{return e.to_string()})
}
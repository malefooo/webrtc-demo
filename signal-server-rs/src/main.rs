
#[macro_use]
extern crate log;
extern crate log4rs;

mod handler;
mod config;

use warp::{
    http::{Response, StatusCode},
    Filter,
};
use std::net::{IpAddr, Ipv4Addr, SocketAddr};
use futures::{FutureExt, TryFutureExt};
use once_cell::sync::OnceCell;
use std::sync::{Arc};
use std::collections::HashMap;
use tokio::sync::{RwLock, mpsc};
use warp::ws::Message;
use std::str::FromStr;

///全局MAP
static KV_MAP: OnceCell<Arc<RwLock<HashMap<String,mpsc::UnboundedSender<Result<Message, warp::Error>>>>>> = OnceCell::new();

/// 使用warp框架来重构了信令服务器
/// 具体使用参考 https://github.com/seanmonstar/warp
///
#[tokio::main]
async fn main() {

    let c = config::C::new();

    let kv_map:Arc<RwLock<HashMap<String, mpsc::UnboundedSender<Result<Message, warp::Error>>>>> = Arc::new(RwLock::new(HashMap::new()));
    KV_MAP.set(kv_map);

    /// websocket连接
    let ws_route = warp::path("ws")
        .and(warp::ws())
        .and(warp::path::param())
        .map(|ws: warp::ws::Ws,domain: String|{
            ws.on_upgrade(move |websocket| handler::ws_handler(websocket,domain))
        });

    /// 发起请求offer处理
    let offer_route = warp::post()
        .and(warp::path("offer"))
        .and(warp::path::end())
        .and(warp::body::json())
        .and_then(handler::offer_handler);

    let routes = ws_route
        .or(offer_route)
        .recover(handler::handle_rejection); //错误处理

    let server = c.server.unwrap();
    let socket = SocketAddr::from_str(server.host.as_str()).unwrap();

    warp::serve(routes).run(socket).await;
}

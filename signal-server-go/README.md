# 信令服务器

## 作用介绍
信令服务器是管理webrtc中peer的websocket连接,同时在对两个peer之间传输所需要的ice-candidate和RemoteDescription

## 接口
1. GET /ws/{domain}
    websocket连接接口,domain是请求者提供;例如answer提供B,供其他offer找到来通信
2. POST /offer body:OfferReq
    peer发起offer,用来获取需要answer的信息接口,信令服务器会通过websocket推送相应数据给请求发起者
   
## 数据结构
 1. OfferReq(offer发给信令服务器的结构体)
   
 | 字段名         | 类型        | 描述                                                         | 属性          |
 | -------------- | ----------- | ------------------------------------------------------------ | ------------- |
 | offer_sdp         | string(base64)      |请求者offer的Description         | required      |
 | candidate | []string      |请求者offer的ice-candidate数组  | required      |
 | answer_domain         | string         |被需要连接的answer在信令服务器注册的域名  | required|
 | offer_domain           | string      |请求者offer在信令服务器注册的域名                 | required      |


 2. Obj(信令服务器推送给offer的数据结构体)

 | 字段名         | 类型        | 描述                                                         | 属性          |
 | -------------- | ----------- | ------------------------------------------------------------ | ------------- |
 | ty         | string      |信令服务器推下去的数据类型,offer/answer         | required      |
 | answer_resp | AnswerResp      |推给offer的是answer_resp  | required      |
 | offer_req         | OfferReq         |推给answer的是offer_req  | required|

 3. AnswerResp(信令服务器推送给offer的数据)
   
 | 字段名         | 类型        | 描述                                                         | 属性          |
 | -------------- | ----------- | ------------------------------------------------------------ | ------------- |
 | answer_sdp         | string(base64)      |被请求者answer的Description         | required      |
 | candidate | []string      |被请求者answer的ice-candidate数组  | required      |
 | answer_domain         | string         |被需要连接的answer在信令服务器注册的域名  | required|
 

 4. Resp(请求信令服务器返回的结构体)
   
 | 字段名         | 类型        | 描述                                                         | 属性          |
 | -------------- | ----------- | ------------------------------------------------------------ | ------------- |
 | code         | int      |0:success,-1:fail         | required      |
 | msg | string      |msg  | required      |
 | data         | T         |数据  | required|

## 配置文件
```toml
#设置信令服务器的地址
[server]
    host = "0.0.0.1:9091"
#配置日志输出的位置和文件
[log]
    out_dir = "/log/signal-server"
    out_file = "signal-server.log"
```
将配置文件放置在和可执行文件一个文件夹下即可
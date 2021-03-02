# 应答节点

## 作用介绍
提供给请求者offer数据,然后在将[]byte转发到grpc服务器上,grpc服务器没有搭建,proto给个示例

## proto数据结构
warpQP是对device的消息进行一层封装，在offer给过来的warpReq，answer会进行拆包然后根据type发送到不同,想对应的grpc-server端,
得到的结果在通过webrtc返回给offer




```protobuf

syntax = "proto3";

import "Device.proto";
package warpQP;
option go_package = "./warpQP;warpQP";

//请求类型的枚举
enum ReqType{
  LoginReq = 0;

  ComputeReq = 1;

  PasswdReq = 2;

  OSSInfoCheck = 3;

  OSSInfoSubmit = 4;
}

//请求的封装体
message WarpReq {

  ReqType type = 1;

  oneof ReqData {
      device.OSSTrueReq oSSTrueReq = 2;

      device.LoginReq loginReq = 3;

      device.ComputeReq computeReq = 4;

      device.PasswdReq passwdReq = 5;

      device.OSSInfo oSSInfo = 6;
  }

}

//返回类型枚举
enum RespType{
  ResultReplyLogin = 0;

  ResultReplyCompute = 1;

  ResultReplyPasswd = 2;

  ResultReplyOSS = 3;
}

//返回的封装体
message WarpResp {
  RespType type = 1;
  oneof RespData {
    device.ResultReplyOSS resultReplyOSS = 2;

    device.ResultReplyLogin resultReplyLogin = 3;

    device.ResultReplyCompute resultReplyCompute = 4;

    device.ResultReplyPasswd resultReplyPasswd = 5;
  }
}

```

## webrtc(offer和answer)和信令服务器交互的数据
1. offerReq(offer发给信令服务器的结构体)

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
 
## 配置文件
```toml
#信令服务器的地址
[server]
    signal_server_host = "127.0.0.1:9091"
    path = "/ws"
    domain = "B"
    device_host = "127.0.0.1:5000"
#stun和turn服务器设置，都必须填写，否则无法启动
[ice_servers]
    [[ice_servers.stun]]
        urls = "sstun:stun.l.google.com:19302"
    [[ice_servers.turn]]
        urls = "turn:192.158.29.39?transport=udp"
        user_name = "unittest"
        credential = "placeholder"
        credential_type = 0 #ICECredentialTypePassword
#日志的输出位置和文件
[log]
    out_dir = "/log/answer"
    out_file = "answer.log"
```
将配置文件放置在和可执行文件一个文件夹下即可
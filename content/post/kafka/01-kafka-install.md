---
title: 'Kafka(Go)系列(二)---通过docker-compose 安装 Kafka'
tags: ["Kafka"]
categories: ["Kafka"]
date: "2021-11-16T20:59:49+08:00"
toc: true
draft: false
---

本文记录了如何通过 docker-compose 快速启动 kafka，部署一套开发环境。

<!--more-->

> 参考修正：https://www.lixueduan.com/post/kafka/01-install/



## 1. 概述

Kafka 是由 Apache 软件基金会旗下的一个开源 `消息引擎系统`。

使用 docker-compose 来部署开发环境也比较方便，只需要提准备一个 yaml 文件即可。

> Kafka 系列相关代码见 [Github](https://github.com/devhg/kafka-go-example)

## 2. docker-compose.yml

完整的 `docker-compose.yaml`内容如下：

> 当前 Kafka 还依赖 Zookeeper，所以需要先启动一个 Zookeeper 。

```yaml
version: "3"
services:
  zookeeper:
    image: 'bitnami/zookeeper:latest'
    ports:
      - '2181:2181'
    environment:
      # 匿名登录--必须开启
      - ALLOW_ANONYMOUS_LOGIN=yes
    #volumes:
      #- ./zookeeper:/bitnami/zookeeper
  # 该镜像具体配置参考 https://github.com/bitnami/bitnami-docker-kafka/blob/master/README.md
  kafka:
    image: 'bitnami/kafka:latest'
    ports:
      - '9092:9092'
      - '9999:9999'
    environment:
      - KAFKA_BROKER_ID=1
      - KAFKA_CFG_ZOOKEEPER_CONNECT=zookeeper:2181
      # 允许使用PLAINTEXT协议(镜像中默认为关闭,需要手动开启)
      - ALLOW_PLAINTEXT_LISTENER=yes
      - KAFKA_CFG_LISTENER_SECURITY_PROTOCOL_MAP=CLIENT:PLAINTEXT,EXTERNAL:PLAINTEXT
      - KAFKA_CFG_LISTENERS=CLIENT://:9093,EXTERNAL://:9092
      - KAFKA_CFG_ADVERTISED_LISTENERS=CLIENT://kafka:9093,EXTERNAL://localhost:9092
      - KAFKA_INTER_BROKER_LISTENER_NAME=CLIENT
      # 开启自动创建 topic 功能
      - KAFKA_CFG_AUTO_CREATE_TOPICS_ENABLE=true
      # 全局消息过期时间 6 小时(测试时可以设置短一点)
      - KAFKA_CFG_LOG_RETENTION_HOURS=6
      # 开启JMX监控
      - JMX_PORT=9999
    #volumes:
      #- ./kafka:/bitnami/kafka
    depends_on:
      - zookeeper
  # Web 管理界面 另外也可以用exporter+prometheus+grafana的方式来监控 https://github.com/danielqsj/kafka_exporter
  kafka_manager:
    image: 'hlebalbau/kafka-manager:latest'
    ports:
      - "9000:9000"
    environment:
      ZK_HOSTS: "zookeeper:2181"
      APPLICATION_SECRET: letmein
    depends_on:
      - zookeeper
      - kafka
```



为了使用内部和外部客户端都能够访问Kafka代理，你需要为每种客户端配置一个侦听器。为此，将以下环境变量添加到docker-compose中

```diff
    environment:
      - KAFKA_CFG_ZOOKEEPER_CONNECT=zookeeper:2181
      - ALLOW_PLAINTEXT_LISTENER=yes
+     - KAFKA_CFG_LISTENER_SECURITY_PROTOCOL_MAP=CLIENT:PLAINTEXT,EXTERNAL:PLAINTEXT
+     - KAFKA_CFG_LISTENERS=CLIENT://:9093,EXTERNAL://:9092
+     - KAFKA_CFG_ADVERTISED_LISTENERS=CLIENT://kafka:9093,EXTERNAL://localhost:9092
+     - KAFKA_INTER_BROKER_LISTENER_NAME=CLIENT
```

`CLIENT://kafka:9093`供内部客户端broker访问。如果外部的客户端需要访问，比如宿主机等其他的机器，我们需要关注这里`EXTERNAL://localhost:9092`，并且需要把9092端口导出，供外部的客户端访问。



KAFKA_CFG_前缀表示kafka的配置，下面是几个配置的作用

- **listeners**：学名叫监听器，其实就是告诉外部连接者要通过什么协议访问指定主机名和端口开放的 Kafka 服务。
- **advertised.listeners**：和 listeners 相比多了个 advertised。Advertised 的含义表示宣称的、公布的，就是说这组监听器是 Broker 用于对外发布的。



监听器它是若干个逗号分隔的三元组，每个三元组的格式为`<协议名称，主机名，端口号>`。

这里的协议名称可能是标准的名字，比如 PLAINTEXT 表示明文传输、SSL 表示使用 SSL 或 TLS 加密传输等；也可能是你自己定义的协议名字。`CLIENT:PLAINTEXT,EXTERNAL:PLAINTEXT`。

一旦你自己定义了协议名称，你必须还要指定`listener.security.protocol.map`参数告诉这个协议底层使用了哪种安全协议，比如上面指定`CLIENT:PLAINTEXT,EXTERNAL:PLAINTEXT`为两个不同的协议，并且在`listeners`和`advertised.listeners`分别进行了配置，其中`KAFKA_INTER_BROKER_LISTENER_NAME=CLIENT`，这个自定义协议底层使用与内部broker通信。



### 镜像

在 dockerhub 上 kafka 相关镜像有 `wurstmeister/kafka` 和 `bitnami/kafka` 这两个用的人比较多,大概看了下 `bitnami/kafka` 更新比较频繁所以就选这个了。

### 监控

监控的话 `hlebalbau/kafka-manager` 这个比较好用，其他的都太久没更新了。

不过 kafka-manager 除了监控外更偏向于集群管理，误操作的话影响比较大，如果有 prometheus + grafana 监控体系的直接用 [kafka_exporter](https://github.com/danielqsj/kafka_exporter) 会舒服很多。

### 数据卷

如果有持久化需求可以放开 yaml 文件中的 `volumes`相关配置，并创建对应文件夹同时将文件夹权限调整为 `777`。

> 因为容器内部使用 uid=1001 的用户在运行程序，容器外部其他用户创建的文件夹对 1001 来说是没有权限的。

### 启动

在 `docker-compose.yaml` 文件目录下使用以下命令即可一键启动：

```bash
docker-compose up
```

## 3. 测试

启动后浏览器直接访问`localhost:9000`即可进入 Web 监控界面。

添加cluster
![](/images/kafka/00-cmk.png)
保存之后，可以看到集群状态，topic信息等
![](/images/kafka/01-cmk.png)

参考

* https://forums.docker.com/t/connecting-kafka-producer-to-kafka-broker-in-docker-through-java/85272/5
* https://github.com/bitnami/bitnami-docker-kafka/blob/master/README.md


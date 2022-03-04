---
title: 'Netty学习系列(一)--ChannelOption中SO_BACKLOG和SO_KEEPALIVE参数'
tags: ['Netty']
categories: ['Netty']
date: "2022-03-04T19:09:49+08:00"
toc: true
draft: false
---

了解下在netty中 ChannelOption.SO_BACKLOG 配置和 ChannelOption.SO_KEEPALIVE 配置的作用分别是什么。

<!--more-->

## ChannelOption.SO_BACKLOG配置

ChannelOption.SO_BACKLOG对应的是tcp/ip协议中 `listen` 函数中的 backlog 参数，用来初始化服务端可连接队列。

函数:

```c
// backlog 指定了内核为此套接口排队的最大连接个数；
// 对于给定的监听套接口，内核要维护两个队列: 未连接队列和已连接队列
// backlog 的值即为未连接队列和已连接队列的和。
listen(int socketfd, int backlog)
```

backlog 用于构造服务端套接字ServerSocket对象，标识当服务器请求处理线程全满时，用于临时存放已完成三次握手的请求的队列的最大长度。

**未连接队列 和 已连接队列**，在linux系统内核中维护了两个队列：`syns queue` 和 `accept queue`

服务端处理客户端连接请求是顺序处理的，所以同一时间只能处理一个客户端连接，多个客户端来的时候，服务端将不能处理的客户端连接请求放在队列中等待处理。



1. syns queue：保存一个SYN已经到达，但三次握手还没有完成的半连接。

用于保存半连接状态的请求，其大小通过`/proc/sys/net/ipv4/tcp_max_syn_backlog`指定，一般默认值是512。不过这个设置有效的前提是系统的syncookies功能被禁用。

> 互联网常见的TCP SYN FLOOD恶意DOS攻击方式就是建立大量的半连接状态的请求，然后丢弃，导致syns queue不能保存其它正常的请求。



2. accept queue：保存三次握手已完成，内核正等待进程执行accept的调用的连接。

用于保存全连接状态的请求，其大小通过`/proc/sys/net/core/somaxconn`指定。在使用listen函数时，内核会根据传入的backlog参数与系统参数somaxconn，取二者的较小值。



注意：

* 如果未设置或所设置的值小于1，Java将使用默认值50。

* 如果accpet queue队列满了，server将发送一个ECONNREFUSED错误信息Connection refused到client。



backlog 设置注意点：

服务器TCP内核 内维护了两个队列，称为A(未连接队列)和B(已连接队列)。如果A+B的长度大于Backlog时，新的连接就会被TCP内核拒绝掉。所以，如果backlog过小，就可能出现Accept的速度跟不上，A，B队列满了，就会导致客户端无法建立连接。


**需要注意的是，backlog对程序的连接数没影响，但是影响的是还没有被Accept取出的 全连接。**



Netty 应用
在netty实现中，backlog默认通过NetUtil.SOMAXCONN指定；也可以在服务器启动启动时，通过option方法自定义backlog的大小。

例如：

```java
// server 启动引导
ServerBootstrap serverBootstrap = new ServerBootstrap();
// 配置启动的参数
serverBootstrap.group(bossGroup,workerGroup)
        // 设置非阻塞,用它来建立新accept的连接,用于构造ServerSocketChannel的工厂类
        .channel(NioServerSocketChannel.class)
        // 临时存放已完成三次握手的请求的队列的最大长度。
        // 如果未设置或所设置的值小于1，Java将使用默认值50。
        // 如果大于队列的最大长度，请求会被拒绝
        .option(ChannelOption.SO_BACKLOG,128)
        .childOption(ChannelOption.SO_KEEPALIVE,true)
        .handler(new ChannelInitializer<SocketChannel>() {
            @Override
            protected void initChannel(SocketChannel ch) throws Exception {

            }
        });
```

![yYRw4J.png](/images/netty/netty-1.png)



## ChannelOption.SO_KEEPALIVE配置

先来了解一下下面的两个问题。

### 1. 为什么需要keepalive

keepalive就是心跳，在网络通信的双方如何证明对端还活着，两个服务之间使用心跳来检测对方是否还活着。

为什么要检测对方是否还活着呢？

假如客户端因为某种原因（停电宕机、终止运行）没有发送关闭连接的数据包，那么服务器就会一直维持着连接，占用服务器资源，后面需要使用连接的时候还会报错。有了心跳，服务器就能及时释放资源。

TCP中的keepalive
TCP keepalive核心参数如下：

```bash
$ sysctl -a| grep tcp_keepalive
net.ipv4.tcp_keepalive_intvl = 75
net.ipv4.tcp_keepalive_probes = 9
net.ipv4.tcp_keepalive_time = 7200
```


TCP在连接没有数据通过后的7200s（tcp_keepalive_time）后会发送keepalive消息，当消息没有被确认后，按75s（tcp_keepalive_intvl）的频率重新发送，一直发送9（tcp_keepalive_probes）个探测包都没有被确认，就认定这个连接失效了。



### 2. 有TCP的keepalive，为什么还需要应用层的keepalive？

* TCP中的keepalive默认是关闭，因为探测包可能在传递过程中会丢失（例如用了代理）。
* 默认的超时时间太长，默认是7200+9*75秒，也就是2个多小时。
* TCP是一个传输层的协议，传输层的数据畅通并不一定操作系统进程所对应的服务畅通。



HTTP中的Keep-Alive是指在HTTP的请求头部携带参数Connection: Keep-Alive，这样浏览器与服务器端就会保持一个长连接，HTTP1.1协议默认是长连接，可以不用携带这个参数。



### Idle检测

Idle是空闲的意思，也就是当客户端不向服务器端发送数据了，不会立马发送心跳包，会等待一段时间，判断这个连接空闲时才会发送。

keepalive的两种设计思路：

1. 开启一个定时任务，不管客户端和服务器端有没有数据的传输，定时发送心跳包。
2. 在连接通道中有数据传送的时候不发送心跳包，无数据传送超过一定时间判定为空闲时再发送。



netty中使用的是第二种。



netty server端开启keepalive：

```java
.childOption(ChannelOption.SO_KEEPALIVE, brokerProperties.getSoKeepAlive());
```

netty server端开启idle：

```java
ch.pipeline().addLast(new IdleStateHandler(0, 20, 0, TimeUnit.SECONDS));
```

IdleStateHandler参数说明：

* readerIdleTime：读空闲时间，超过指定时间未读取数据就会触发IdleState.READER_IDLE事件。
* writerIdleTime：写空闲时间，超过指定时间未发送数据就会触发IdleState.WRITER_IDLE事件。
* allIdleTime：读或写空闲时间，超过指定时间未读取或者发送数据就会触发IdleState.ALL_IDLE事件。
* unit：时间单位。



### 实验样例

服务器端的代码实现：服务器端开启读空闲监测，60s未收到数据就会关闭连接。

```java
package netty.keepalive;

import io.netty.bootstrap.ServerBootstrap;
import io.netty.channel.*;
import io.netty.channel.nio.NioEventLoopGroup;
import io.netty.channel.socket.SocketChannel;
import io.netty.channel.socket.nio.NioChannelOption;
import io.netty.channel.socket.nio.NioServerSocketChannel;
import io.netty.handler.codec.LineBasedFrameDecoder;
import io.netty.handler.codec.string.StringDecoder;
import io.netty.handler.codec.string.StringEncoder;
import io.netty.handler.logging.LoggingHandler;
import io.netty.handler.timeout.IdleState;
import io.netty.handler.timeout.IdleStateEvent;
import io.netty.handler.timeout.IdleStateHandler;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.net.StandardSocketOptions;
import java.util.concurrent.TimeUnit;

public class Server {
    private static final Logger LOGGER = LoggerFactory.getLogger(Server.class);

    public static final int PORT = 8899;

    public static void main(String[] args) throws InterruptedException {
        EventLoopGroup bossGroup = new NioEventLoopGroup();
        EventLoopGroup workerGroup = new NioEventLoopGroup();
        try {
            ServerBootstrap b = new ServerBootstrap();
            b.group(bossGroup, workerGroup)
                    .channel(NioServerSocketChannel.class)
                    .childOption(ChannelOption.SO_KEEPALIVE, true)
                    .childOption(NioChannelOption.of(StandardSocketOptions.SO_KEEPALIVE), true)
                    .childHandler(new ChannelInitializer<SocketChannel>() {
                        @Override
                        public void initChannel(SocketChannel ch) throws Exception {
                            ch.pipeline().addLast(new LoggingHandler());
                            ch.pipeline().addLast(new IdleStateHandler(60, 0, 0, TimeUnit.SECONDS));
                            ch.pipeline().addLast(new LineBasedFrameDecoder(1 << 10));
                            ch.pipeline().addLast(new StringDecoder());
                            ch.pipeline().addLast(new StringEncoder());
                            ch.pipeline().addLast(new SimpleChannelInboundHandler<String>() {
                                @Override
                                protected void channelRead0(ChannelHandlerContext ctx, String msg) throws Exception {
                                    LOGGER.info("receive from client: {}", msg);
                                    ctx.writeAndFlush("ok\n");
                                }

                                @Override
                                public void userEventTriggered(ChannelHandlerContext ctx, Object evt) throws Exception {
                                    if (evt instanceof IdleStateEvent) {
                                        IdleStateEvent idleStateEvent = (IdleStateEvent) evt;
                                        if (idleStateEvent.state() == IdleState.READER_IDLE) {
                                            // 60s未收到数据就会关闭连接
                                            LOGGER.warn("timeout: {}", ctx.channel().remoteAddress());
                                            ctx.channel().close();
                                        }
                                    } else {
                                        super.userEventTriggered(ctx, evt);
                                    }
                                }

                                @Override
                                public void exceptionCaught(ChannelHandlerContext ctx, Throwable cause) {
                                    cause.printStackTrace();
                                    ctx.close();
                                }
                            });
                        }
                    });

            // 启动 server.
            ChannelFuture f = b.bind(PORT).sync();
            System.out.println("server is start on port: " + PORT);

            // 等待socket关闭
            f.channel().closeFuture().sync();
        } finally {
            workerGroup.shutdownGracefully();
            bossGroup.shutdownGracefully();
        }
    }
}

```

客户端代码的实现：客户端开启写空闲监测，30s未写数据就会发送心跳。

```java
package netty.keepalive;

import io.netty.bootstrap.Bootstrap;
import io.netty.channel.*;
import io.netty.channel.nio.NioEventLoopGroup;
import io.netty.channel.socket.SocketChannel;
import io.netty.channel.socket.nio.NioSocketChannel;
import io.netty.handler.codec.LineBasedFrameDecoder;
import io.netty.handler.codec.string.StringDecoder;
import io.netty.handler.codec.string.StringEncoder;
import io.netty.handler.logging.LoggingHandler;
import io.netty.handler.timeout.IdleState;
import io.netty.handler.timeout.IdleStateEvent;
import io.netty.handler.timeout.IdleStateHandler;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.concurrent.TimeUnit;

public class Client {
    private static final Logger LOGGER = LoggerFactory.getLogger(Client.class);

    public static void main(String[] args) throws InterruptedException {
        EventLoopGroup workerGroup = new NioEventLoopGroup();
        try {
            Bootstrap b = new Bootstrap();
            b.group(workerGroup)
                    .channel(NioSocketChannel.class)
                    .handler(new ChannelInitializer<SocketChannel>() {
                        @Override
                        public void initChannel(SocketChannel ch) throws Exception {
                            ch.pipeline().addLast(new LoggingHandler());
                            ch.pipeline().addLast(new IdleStateHandler(0, 30, 0, TimeUnit.SECONDS));
                            ch.pipeline().addLast(new LineBasedFrameDecoder(1 << 10));
                            ch.pipeline().addLast(new StringEncoder());
                            ch.pipeline().addLast(new StringDecoder());
                            ch.pipeline().addLast(new SimpleChannelInboundHandler<String>() {
                                @Override
                                public void channelActive(ChannelHandlerContext ctx) throws Exception {
                                    ctx.writeAndFlush("hello\n");
                                }

                                @Override
                                public void channelRead0(ChannelHandlerContext ctx, String msg) {
                                    LOGGER.info("receive from server: {}", msg);
                                }

                                @Override
                                public void userEventTriggered(ChannelHandlerContext ctx, Object evt) throws Exception {
                                    if (evt instanceof IdleStateEvent) {
                                        IdleStateEvent idleStateEvent = (IdleStateEvent) evt;
                                        if (idleStateEvent.state() == IdleState.WRITER_IDLE) {
                                            // 30s未写数据就会发送心跳
                                            ctx.writeAndFlush("hi\n");
                                        }
                                    } else {
                                        super.userEventTriggered(ctx, evt);
                                    }
                                }

                                @Override
                                public void exceptionCaught(ChannelHandlerContext ctx, Throwable cause) {
                                    cause.printStackTrace();
                                    ctx.close();
                                }
                            });
                        }
                    });

            // 启动 server.
            ChannelFuture f = b.connect("127.0.0.1", 8899).sync();

            // 等待socket关闭
            f.channel().closeFuture().sync();
        } finally {
            workerGroup.shutdownGracefully();
        }
    }
}

```



当然正常情况下本地测试上面的代码，服务器端是不会触发读空闲的事件，即使强制关闭了客户端，客户端也会发送关闭连接的请求给服务器，然后服务器端将连接关闭。

服务器端要想触发读空闲的事件，可以在使用两台机器或者虚拟机来测试，客户端启动后直接把网线拔出，如果是在linux下可以使用下面的命令关闭网络来测试：

```
# ifconfig ens32 down // 关闭网卡
# ifconfig ens32 up   // 开启网卡
```


---
title: 'Kafka(Go)系列(三)---Producer-Consumer API 基本使用'
tags: ["Kafka"]
categories: ["Kafka"]
date: "2021-11-17T21:20:39+08:00"
toc: true
draft: false
---



<!--more-->

> 复现参考修正于：https://www.lixueduan.com/post/kafka/05-quick-start/
>
> 复现修正的代码：https://github.com/devhg/kafka-go-example



## 1. Producer API

Kafka 中生产者分为同步生产者和异步生产者。

顾名思义，同步生产者每条消息都会实时发送到 Kafka，而异步生产者则为了提升性能，会等待存了一批消息或者到了指定间隔时间才会一次性发送到 Kafka。

### 1.1 Async Producer

sarama 中异步生产者使用 Demo 如下

`````go
// 本例展示最简单的 异步生产者 的使用（除异步生产者外 kafka 还有同步生产者）

// 统计生产者发送的消息数量
var count int64

func Producer(topic string, limit int) {
	config := sarama.NewConfig()
	// 异步生产者不建议把 Errors 和 Successes 都开启，一般开启 Errors 就行
	// 同步生产者就必须都开启，因为会同步返回发送成功或者失败
	config.Producer.Return.Errors = true    // 设定是否需要返回错误信息
	config.Producer.Return.Successes = true // 设定是否需要返回成功信息

	producer, err := sarama.NewAsyncProducer([]string{conf.HOST}, config)
	if err != nil {
		log.Fatal("NewAsyncProducer err:", err)
	}
	defer producer.AsyncClose()

	go func() {
		// 采用Timer 而不是使用time.After 原因：time.After会产生内存泄漏 在计时器触发之前，垃圾回收器不会回收Timer
		t := time.NewTimer(time.Minute * 1)
		defer t.Stop()
		for {
			// [!important] 异步生产者发送后必须把返回值从 Errors 或者 Successes 中读出来,
			// 不然会阻塞 sarama 内部处理逻辑 导致只能发出去一条消息
			select {
			case suc := <-producer.Successes():
				if suc != nil {
					// log.Printf("[Producer] Success: key:%v msg:%+v \n", suc.Key, suc.Value)
				}
			case fail := <-producer.Errors():
				if fail != nil {
					log.Printf("[Producer] Errors: err:%v msg:%+v \n", fail.Err, fail.Msg)
				}
			case <-t.C:
				return
			}

			if !t.Stop() {
				t.Reset(time.Minute * 1)
			}
		}
	}()

	// 异步生产者发送消息
	for i := 0; i < limit; i++ {
		str := strconv.Itoa(int(time.Now().UnixNano()))
		msg := &sarama.ProducerMessage{
			Topic: topic,
			Value: sarama.StringEncoder(str),
		}

		// 异步发送只是写入内存了就返回了，并没有真正发送出去
		// sarama 库中用的是一个 channel 来接收，后台 goroutine 异步从该 channel 中取出消息并真正发送
		// 因此，具体的响应包括 Success 或者 Errors 也是通过 chan 异步返回的。
		producer.Input() <- msg

		atomic.AddInt64(&count, 1)
		if atomic.LoadInt64(&count)%1000 == 0 {
			log.Printf("已发送消息数:%v\n", count)
		}
	}
	log.Printf("发送完毕 总发送消息数:%v\n", limit)
}

`````

注意点：

异步生产者只需要将消息发送到 chan 就会返回，具体的响应包括 Success 或者 Errors 是通过 chan 异步返回的。

**必须把返回值从 Errors 或者 Successes 中读出来 不然会阻塞 producer.Input()**



### 1.2 Sync Producer

同步生产者就更简单了：

```go
// 本例展示最简单的 同步生产者 的使用（除同步生产者外 kafka 还有异步生产者）
func Producer(topic string, limit int) {
	config := sarama.NewConfig()

	// 同步生产者必须同时开启 Return.Successes 和 Return.Errors
	// 因为同步生产者在发送之后就必须返回状态，所以需要两个都返回
	config.Producer.Return.Successes = true
	config.Producer.Return.Errors = true

	// 同步生产者和异步生产者逻辑是一致的，Success或者Errors都是通过channel返回的，
	// 只是同步生产者封装了一层，等channel返回之后才返回给调用者
	// 具体见 sync_producer.go 文件72行 newSyncProducerFromAsyncProducer 方法
	// 内部启动了两个 goroutine 分别处理Success Channel 和 Errors Channel
	// 同步生产者内部就是封装的异步生产者
	producer, err := sarama.NewSyncProducer([]string{conf.HOST}, config)
	if err != nil {
		log.Fatal("NewSyncProducer err:", err)
	}
	defer producer.Close()

	for i := 0; i < limit; i++ {
		str := strconv.Itoa(int(time.Now().UnixNano()))
		msg := &sarama.ProducerMessage{
			Topic: conf.Topic,
			Key:   nil,
			Value: sarama.StringEncoder(str),
		}

		// 发送逻辑也是封装的异步发送逻辑，可以理解为将异步封装成了同步
		partition, offset, err := producer.SendMessage(msg)
		if err != nil {
			log.Println("SendMessage err: ", err)
			return
		}
		log.Printf("[Producer] partitionID: %d; offset:%d, value: %s\n", partition, offset, str)
	}
}
```

注意点：

**必须同时开启 Return.Successes 和 Return.Errors**



## 2. Consumer API

Kafka 中消费者分为独立消费者和消费者组。

### 2.1 StandaloneConsumer

```go
// SinglePartition 单分区消费
func SinglePartition(topic string) {
	config := sarama.NewConfig()
	consumer, err := sarama.NewConsumer([]string{conf.HOST}, config)
	if err != nil {
		log.Fatal("NewConsumer err: ", err)
	}
	defer consumer.Close()

	// 参数1 指定消费哪个 topic
	// 参数2 分区 这里默认消费 0 号分区 kafka 中有分区的概念，类似于ES和MongoDB中的sharding，MySQL中的分表这种
	// 参数3 offset 从哪儿开始消费起走，正常情况下每次消费完都会将这次的offset提交到kafka，然后下次可以接着消费，
	// 这里 sarama.OffsetNewest 就从最新的开始消费，即该 consumer 启动之前产生的消息都无法被消费
	// 如果改为 sarama.OffsetOldest 则会从最旧的消息开始消费，即每次重启 consumer 都会把该 topic 下的所有消息消费一次
	partitionConsumer, err := consumer.ConsumePartition(topic, 0, sarama.OffsetOldest)
	if err != nil {
		log.Fatal("ConsumePartition err: ", err)
	}
	defer partitionConsumer.Close()

	// 会一直阻塞在这里
	for message := range partitionConsumer.Messages() {
		log.Printf("[Consumer] partitionID: %d; offset:%d, value: %s\n",
			message.Partition, message.Offset, string(message.Value))
	}
}
```

更多祥见。

反复运行上面的 Demo 会发现，每次都会从第 1 条消息开始消费，或者当使用`sarama.OffsetNewest`的时候，只要没有新的消息，就会一直阻塞。

Kafka 和其他 MQ 最大的区别在于 Kafka 中的消息在消费后不会被删除，而是会一直保留，直到过期才会被删除。

为了防止每次重启消费者都从第 1 条消息开始消费，**我们需要在消费消息后将 offset 提交给 Kafka**。这样重启后就可以接着上次的 Offset 继续消费了。

### 2.2 OffsetManager

在独立消费者中没有实现提交 Offset 的功能，所以我们需要借助 OffsetManager 来完成。

```go
func OffsetManager(topic string) {
	config := sarama.NewConfig()
	// 配置开启自动提交 offset，这样 samara 库会定时帮我们把最新的 offset 信息提交给 kafka
	config.Consumer.Offsets.AutoCommit.Enable = true              // 开启自动 commit offset
	config.Consumer.Offsets.AutoCommit.Interval = 1 * time.Minute // 每 1 分钟提交一次 offset

	client, err := sarama.NewClient([]string{conf.HOST}, config)
	if err != nil {
		log.Fatal("NewClient err: ", err)
	}
	defer client.Close()

	// offsetManager 用于管理每个consumerGroup的 offset
	// 根据 groupID 来区分不同的 consumer，注意: 每次提交的 offset 信息也是和 groupID 关联的
	offsetManager, err := sarama.NewOffsetManagerFromClient("myGroupID", client)
	if err != nil {
		log.Fatal("NewOffsetManagerFromClient err: ", err)
	}
	defer offsetManager.Close()

	// 每个分区的 offset 也是分别管理的，这里使用 0 分区，因为该 topic 只有 1 个分区
	partitionOffsetManager, err := offsetManager.ManagePartition(topic, conf.DefaultPartition)
	if err != nil {
		log.Fatal("ManagePartition err: ", err)
	}
	defer partitionOffsetManager.Close()
	defer offsetManager.Commit() // defer 在程序结束后在 commit 一次，防止自动提交间隔之间的信息被丢掉

	consumer, err := sarama.NewConsumerFromClient(client)
	if err != nil {
		log.Fatal("NewConsumerFromClient err: ", err)
	}

	// 根据 kafka 中记录的上次消费的 offset 开始+1的位置接着消费
	nextOffset, _ := partitionOffsetManager.NextOffset()
	fmt.Println("nextOffset:", nextOffset)

	// nextOffset = 500  // 手动设置下次消费的 offset

	pc, err := consumer.ConsumePartition(topic, conf.DefaultPartition, nextOffset)
	if err != nil {
		log.Fatal("ConsumePartition err: ", err)
	}
	defer pc.AsyncClose()

	// 开始消费
	for msg := range pc.Messages() {
		log.Printf("[Consumer] partitionID: %d; offset:%d, value: %s\n",
			msg.Partition, msg.Offset, string(msg.Value))
		// 每次消费后都更新一次 offset, 这里更新的只是程序内存中的值，需要 commit 之后才能提交到 kafka
		partitionOffsetManager.MarkOffset(msg.Offset+1, "modified metadata")
	}
}

```



### 2.3 ConsumerGroup

Kafka 消费者组中可以存在多个消费者，**Kafka 会以 partition 为单位将消息分给各个消费者**。**每条消息只会被消费者组的一个消费者消费**。

> 注意：是以分区为单位，如果消费者组中有两个消费者，但是订阅的 Topic 只有 1 个分区，那么注定有一个消费者永远消费不到任何消息。

消费者组的好处在于并发消费，Kafka 把分发逻辑已经实现了，我们只需要启动多个消费者即可。

> 如果只有一个消费者，我们需要手动获取消息后分发给多个 Goroutine，需要多写一段代码，而且 Offset 维护还比较麻烦。

```go
// Consumer 实现 sarama.ConsumerGroupHandler 接口，作为自定义ConsumerGroup
type Consumer struct {
	name  string
	count int64
	ready chan bool
}

// Setup 执行在 获得新 session 后 的第一步, 在 ConsumeClaim() 之前
func (c *Consumer) Setup(_ sarama.ConsumerGroupSession) error {
	fmt.Println(c.name, "Setup")
	c.ready <- true
	return nil
}

// Cleanup 执行在 session 结束前, 当所有 ConsumeClaim goroutines 都退出时
func (c *Consumer) Cleanup(_ sarama.ConsumerGroupSession) error {
	fmt.Println(c.name, "Count", c.count)
	return nil
}

// ConsumeClaim 具体的消费逻辑
func (c *Consumer) ConsumeClaim(sess sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		log.Printf("[consumer] name:%s topic:%q partition:%d offset:%d\n",
			c.name, msg.Topic, msg.Partition, msg.Offset)
		// 标记消息已被消费 内部会更新 consumer offset
		sess.MarkMessage(msg, "")
		c.count++
	}
	return nil
}

func ConsumerGroup(topic, group, name string) {
	config := sarama.NewConfig()
	config.Consumer.Return.Errors = true

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 消费者组
	cg, err := sarama.NewConsumerGroup([]string{conf.HOST}, group, config)
	if err != nil {
		log.Fatal("NewConsumerGroup err:", err)
	}
	defer cg.Close()

	// 创建一个消费者组的消费者
	handler := &Consumer{name: name, ready: make(chan bool)}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			fmt.Println("running: ", name)

			// 应该在一个无限循环中不停地调用 Consume()
			// 因为每次 Rebalance 后需要再次执行 Consume() 来恢复连接
			// Consume 开始才发起 Join Group 请求。如果当前消费者加入后成为了 消费者组 leader,
			// 则还会进行 Rebalance 过程，从新分配组内每个消费组需要消费的 topic 和 partition，
			// 最后 Sync Group 后才开始消费
			err = cg.Consume(ctx, []string{topic}, handler)
			if err != nil {
				log.Fatal("Consume err: ", err)
			}
			// 如果 context 被 cancel 了，那么退出
			if ctx.Err() != nil {
				fmt.Println(ctx.Err())
				return
			}
		}
	}()

	<-handler.ready

	wg.Wait()
	if err = cg.Close(); err != nil {
		log.Panicf("Error closing client: %v", err)
	}
}

```

注意。在实现接口的`ConsumeClaim`方法中，需要调用`sess.MarkMessage()`方法更新 Offset。

## 3. 总结

生产者

- 同步生产者：同步发送，效率低实时性高
- 异步生产者：1. 异步发送，效率高   2. 消息大小、数量达到阈值或间隔时间达到设定值时触发发送。异步生产者不会阻塞，而且会批量发送消息给 Kafka，性能上优于 同步生产者。

消费者

- 独立消费者：需要配合 OffsetManager 使用
- 消费者组：1. 以分区为单位将消息分发给组里的各个消费者。2. 若消费者数大于分区数，必定有消费者消费不到消息






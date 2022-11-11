---
title: 'Go语言之 sync.Mutex互斥锁'
tags: ["Go"]
categories: ["Go"]
date: "2021-12-23T15:04:50+08:00"
toc: true
draft: false
---

go语言以并发作为其特性之一，并发必然会带来对于资源的竞争，这时候我们就需要使用go提供的`sync.Mutex`这把互斥锁来保证临界资源的访问互斥。

既然经常会用这把锁，那么了解一下其内部实现，就能了解这把锁适用什么场景，特性如何了。

<!--more-->



## 引子

在我第一次看这段代码的时候，感觉真的是惊为天人，特别是整个`Mutex`只用到了两个私有字段，以及一次CAS就加锁的过程，这其中设计以及编程的理念真的让我感觉自愧不如。

在看`sync.Mutex`的代码的时候，一定要记住，同时会有多个goroutine会来要这把锁，所以锁的状态`state`是可能会一直更改的。



## 锁的性质

先说结论：`sync.Mutex`是把公平锁。

在源代码中，有一段注释：

```go
// Mutex fairness.
//
// Mutex can be in 2 modes of operations: normal and starvation.
// In normal mode waiters are queued in FIFO order, but a woken up waiter
// does not own the mutex and competes with new arriving goroutines over
// the ownership. New arriving goroutines have an advantage -- they are
// already running on CPU and there can be lots of them, so a woken up
// waiter has good chances of losing. In such case it is queued at front
// of the wait queue. If a waiter fails to acquire the mutex for more than 1ms,
// it switches mutex to the starvation mode.
//
// In starvation mode ownership of the mutex is directly handed off from
// the unlocking goroutine to the waiter at the front of the queue.
// New arriving goroutines don't try to acquire the mutex even if it appears
// to be unlocked, and don't try to spin. Instead they queue themselves at
// the tail of the wait queue.
//
// If a waiter receives ownership of the mutex and sees that either
// (1) it is the last waiter in the queue, or (2) it waited for less than 1 ms,
// it switches mutex back to normal operation mode.
//
// Normal mode has considerably better performance as a goroutine can acquire
// a mutex several times in a row even if there are blocked waiters.
// Starvation mode is important to prevent pathological cases of tail latency.复制代码
```

看懂这段注释对于我们理解mutex这把锁有很大的帮助，这里面讲了这把锁的设计理念。大致意思如下：

```go

```

在下一步真正看源代码之前，我们必须要理解一点：

当一个goroutine获取到锁的时候，有可能没有竞争者，也有可能会有很多竞争者，那么我们就需要站在不同的goroutine的角度上去考虑goroutine看到的锁的状态和实际状态、期望状态之间的转化。



## Mutex结构体

`sync.Mutex`只包含两个字段：

```go
// A Mutex is a mutual exclusion lock.
// The zero value for a Mutex is an unlocked mutex.
//
// A Mutex must not be copied after first use.
type Mutex struct {
    state int32
    sema  uint32
}

const (
    mutexLocked = 1 << iota // mutex is locked
    mutexWoken
    mutexStarving
    mutexWaiterShift = iota

    starvationThresholdNs = 1e6
)
```

其中`state`是一个表示锁的状态的字段，这个字段会同时被多个goroutine所共用（使用atomic.CAS来保证原子性），第0个bit（1）表示锁已被获取，也就是已加锁，被某个goroutine拥有；第1个bit（2）表示有goroutine被唤醒，尝试获取锁；第2个bit（4）标记这把锁是否为饥饿状态。

`sema`字段就是用来唤醒goroutine所用的信号量。



## Lock

在看代码之前，我们需要有一个概念：每个goroutine也有自己的状态，存在局部变量里面（也就是函数栈里面），goroutine有可能是新到的、被唤醒的、正常的、饥饿的。

### atomic.CAS

先瞻仰一下惊为天人的一行代码加锁的CAS操作：

```go
// Lock locks m.
// If the lock is already in use, the calling goroutine
// blocks until the mutex is available.
func (m *Mutex) Lock() {
    // Fast path: grab unlocked mutex.
    if atomic.CompareAndSwapInt32(&m.state, 0, mutexLocked) {
        if race.Enabled {
            race.Acquire(unsafe.Pointer(m))
        }
        return
    }
   	// Slow path (outlined so that the fast path can be inlined)
	m.lockSlow()
}
```

这是第一段代码，这段代码调用了`atomic`包中的`CompareAndSwapInt32`这个方法来尝试快速获取锁，这个方法的签名如下：

```go
// CompareAndSwapInt32 executes the compare-and-swap operation for an int32 value.
func CompareAndSwapInt32(addr *int32, old, new int32) (swapped bool)
```

意思是，如果addr指向的地址中存的值和old一样，那么就把addr中的值改为new并返回true；否则什么都不做，返回false。由于是`atomic`中的函数，所以是保证了原子性的，底层由特定的汇编指令实现。

我们来具体看看CAS的实现（`src/runtime/internal/atomic/asm_amd64.s`）：

```assembly
// bool Cas(int32 *val, int32 old, int32 new)
// Atomically:
//    if(*val == old){
//        *val = new;
//        return 1;
//    } else
//        return 0;
// 这里参数及返回值大小加起来是17，是因为一个指针在amd64下是8字节，
// 然后int32分别是占用4字节，最后的返回值是bool占用1字节，所以加起来是17
TEXT runtime∕internal∕atomic·Cas(SB),NOSPLIT,$0-17 
    // 为什么不把*val指针放到AX中呢？因为AX有特殊用处，
    // 在下面的CMPXCHGL里面，会从AX中读取要比较的其中一个数
    MOVQ    ptr+0(FP), BX
    // 所以AX要用来存参数old
    MOVL    old+8(FP), AX
    // 把new中的数存到寄存器CX中
    MOVL    new+12(FP), CX
    // 注意这里了，这里使用了LOCK前缀，所以保证操作是原子的
    LOCK
    // 0(BX) 可以理解为 *val
    // 把 AX中的数 和 第二个操作数 0(BX)——也就是BX寄存器所指向的地址中存的值 进行比较
    // 如果相等，就把 第一个操作数 CX寄存器中存的值 赋给 第二个操作数 BX寄存器所指向的地址
    // 并将标志寄存器ZF设为1
    // 否则将标志寄存器ZF清零
    CMPXCHGL    CX, 0(BX)
    // SETE的作用是：
    // 如果Zero Flag标志寄存器为1，那么就把操作数设为1
    // 否则把操作数设为0
    // 也就是说，如果上面的比较相等了，就返回true，否则为false
    // ret+16(FP)代表了返回值的地址
    SETEQ    ret+16(FP)
    RET
```

如果看不懂也没太大关系，只要知道这个函数的作用，以及这个函数是原子性的即可。

那么这段代码的意思就是：先看看这把锁是不是空闲状态，如果是的话，直接原子性地修改一下`state`为已被获取就行了。多么简洁（虽然后面的代码并不是……）！

### 主流程

接下来具体看主流程的代码，代码中有一些位运算看起来比较晕，我会试着用伪代码在边上注释。

```go
func (m *Mutex) lockSlow() {
    // 用来存当前goroutine的等待时间
	var waitStartTime int64
    // 用来存当前goroutine是否饥饿
	starving := false
    // 用来存当前goroutine是否已唤醒
	awoke := false
    // 用来存当前goroutine的循环自旋次数 (想一想一个goroutine如果循环了2147483648次咋办……)
	iter := 0
    // 复制存放一下当前锁的状态
	old := m.state
    // 自旋
	for {
		// 如果是饥饿情况之下，就不要自旋了，因为锁会直接交给队列头部的goroutine
        // 如果锁是被获取状态，并且满足自旋条件（canSpin见后文分析），那么就自旋等锁
        // 伪代码：if isLocked() and isNotStarving() and canSpin()
		if old&(mutexLocked|mutexStarving) == mutexLocked && runtime_canSpin(iter) {
			// 将自己的状态以及锁的状态设置为唤醒，这样当Unlock的时候就不会去唤醒其它被阻塞的goroutine了
			if !awoke && old&mutexWoken == 0 && old>>mutexWaiterShift != 0 &&
				atomic.CompareAndSwapInt32(&m.state, old, old|mutexWoken) {
				awoke = true
			}
            //自旋
			runtime_doSpin()
			iter++
            // 更新锁的状态(有可能在自旋的这段时间之内锁的状态已经被其它goroutine改变
			old = m.state
			continue
		}
		new := old
		// Don't try to acquire starving mutex, new arriving goroutines must queue.
		if old&mutexStarving == 0 {
			new |= mutexLocked
		}
		if old&(mutexLocked|mutexStarving) != 0 {
			new += 1 << mutexWaiterShift
		}
		// The current goroutine switches mutex to starvation mode.
		// But if the mutex is currently unlocked, don't do the switch.
		// Unlock expects that starving mutex has waiters, which will not
		// be true in this case.
		if starving && old&mutexLocked != 0 {
			new |= mutexStarving
		}
		if awoke {
			// The goroutine has been woken from sleep,
			// so we need to reset the flag in either case.
			if new&mutexWoken == 0 {
				throw("sync: inconsistent mutex state")
			}
			new &^= mutexWoken
		}
		if atomic.CompareAndSwapInt32(&m.state, old, new) {
			if old&(mutexLocked|mutexStarving) == 0 {
				break // locked the mutex with CAS
			}
			// If we were already waiting before, queue at the front of the queue.
			queueLifo := waitStartTime != 0
			if waitStartTime == 0 {
				waitStartTime = runtime_nanotime()
			}
			runtime_SemacquireMutex(&m.sema, queueLifo, 1)
			starving = starving || runtime_nanotime()-waitStartTime > starvationThresholdNs
			old = m.state
			if old&mutexStarving != 0 {
				// If this goroutine was woken and mutex is in starvation mode,
				// ownership was handed off to us but mutex is in somewhat
				// inconsistent state: mutexLocked is not set and we are still
				// accounted as waiter. Fix that.
				if old&(mutexLocked|mutexWoken) != 0 || old>>mutexWaiterShift == 0 {
					throw("sync: inconsistent mutex state")
				}
				delta := int32(mutexLocked - 1<<mutexWaiterShift)
				if !starving || old>>mutexWaiterShift == 1 {
					// Exit starvation mode.
					// Critical to do it here and consider wait time.
					// Starvation mode is so inefficient, that two goroutines
					// can go lock-step infinitely once they switch mutex
					// to starvation mode.
					delta -= mutexStarving
				}
				atomic.AddInt32(&m.state, delta)
				break
			}
			awoke = true
			iter = 0
		} else {
			old = m.state
		}
	}

	if race.Enabled {
		race.Acquire(unsafe.Pointer(m))
	}
}
```

以上为什么CAS能拿到锁呢？因为CAS会原子性地判断`old state`和当前锁的状态是否一致；而总有一个goroutine会满足以上条件成功拿锁。

### canSpin

接下来我们来看看上文提到的`canSpin`条件如何：

```
// Active spinning for sync.Mutex.
//go:linkname sync_runtime_canSpin sync.runtime_canSpin
//go:nosplit
func sync_runtime_canSpin(i int) bool {
    // 这里的active_spin是个常量，值为4
    // 简单来说，sync.Mutex是有可能被多个goroutine竞争的，所以不应该大量自旋(消耗CPU)
    // 自旋的条件如下：
    // 1. 自旋次数小于active_spin(这里是4)次；
    // 2. 在多核机器上；
    // 3. GOMAXPROCS > 1并且至少有一个其它的处于运行状态的P；
    // 4. 当前P没有其它等待运行的G；
    // 满足以上四个条件才可以进行自旋。
    if i >= active_spin || ncpu <= 1 || gomaxprocs <= int32(sched.npidle+sched.nmspinning)+1 {
        return false
    }
    if p := getg().m.p.ptr(); !runqempty(p) {
        return false
    }
    return true
}复制代码
```

所以可以看出来，并不是一直无限自旋下去的，当自旋次数到达4次或者其它条件不符合的时候，就改为信号量拿锁了。

### doSpin

然后我们来看看`doSpin`的实现（其实也没啥好看的）：

```
//go:linkname sync_runtime_doSpin sync.runtime_doSpin
//go:nosplit
func sync_runtime_doSpin() {
    procyield(active_spin_cnt)
}复制代码
```

这是一个汇编实现的函数，简单看两眼amd64上的实现：

```
TEXT runtime·procyield(SB),NOSPLIT,$0-0
    MOVL    cycles+0(FP), AX
again:
    PAUSE
    SUBL    $1, AX
    JNZ    again
    RET复制代码
```

看起来没啥好看的，直接跳过吧。

## Unlock

接下来我们来看看Unlock的实现，对于Unlock来说，有两个比较关键的特性：

1. 如果说锁不是处于locked状态，那么对锁执行Unlock会导致panic；
2. 锁和goroutine没有对应关系，所以我们完全可以在goroutine 1中获取到锁，然后在goroutine 2中调用Unlock来释放锁（这是什么骚操作！）（虽然不推荐大家这么干……）

```
func (m *Mutex) Unlock() {
    if race.Enabled {
        _ = m.state
        race.Release(unsafe.Pointer(m))
    }

    // Fast path: drop lock bit.
    // 这里获取到锁的状态，然后将状态减去被获取的状态(也就是解锁)，称为new(期望)状态
    // 注意以上两个操作是原子的，所以不用担心多个goroutine并发的问题
    new := atomic.AddInt32(&m.state, -mutexLocked)
    // 如果说，期望状态加上被获取的状态，不是被获取的话
    // 那么就panic
    // 在这里给大家提一个问题：干嘛要这么大费周章先减去再加上，直接比较一下原来锁的状态是否被获取不就完事了？
    if (new+mutexLocked)&mutexLocked == 0 {
        throw("sync: unlock of unlocked mutex")
    }
    // 如果说new状态(也就是锁的状态)不是饥饿状态
    if new&mutexStarving == 0 {
        // 复制一下原先状态
        old := new
        for {
            // 如果说锁没有等待拿锁的goroutine
            // 或者锁被获取了(在循环的过程中被其它goroutine获取了)
            // 或者锁是被唤醒状态(表示有goroutine被唤醒，不需要再去尝试唤醒其它goroutine)
            // 或者锁是饥饿模式(会直接转交给队列头的goroutine)
            // 那么就直接返回，啥都不用做了
            if old>>mutexWaiterShift == 0 || old&(mutexLocked|mutexWoken|mutexStarving) != 0 {
                return
            }
            // 走到这一步的时候，说明锁目前还是空闲状态，并且没有goroutine被唤醒且队列中有goroutine等待拿锁
            // 那么我们就要把锁的状态设置为被唤醒，等待队列-1
            new = (old - 1<<mutexWaiterShift) | mutexWoken
            // 又是熟悉的CAS
            if atomic.CompareAndSwapInt32(&m.state, old, new) {
                // 如果状态设置成功了，我们就通过信号量去唤醒goroutine
                runtime_Semrelease(&m.sema, false)
                return
            }
            // 循环结束的时候，更新一下状态，因为有可能在执行的过程中，状态被修改了(比如被Lock改为了饥饿状态)
            old = m.state
        }
    } else {
        // 如果是饥饿状态下，那么我们就直接把锁的所有权通过信号量移交给队列头的goroutine就好了
        // handoff = true表示直接把锁交给队列头部的goroutine
        // 注意：在这个时候，锁被获取的状态没有被设置，会由被唤醒的goroutine在唤醒后设置
        // 但是当锁处于饥饿状态的时候，我们也认为锁是被获取的(因为我们手动指定了获取的goroutine)
        // 所以说新来的goroutine不会尝试去获取锁(在Lock中有体现)
        runtime_Semrelease(&m.sema, true)
    }
}复制代码
```



## 总结

根据以上代码的分析，可以看出，`sync.Mutex`这把锁在你的工作负载（所需时间）比较低，比如只是对某个关键变量赋值的时候，性能还是比较好的，但是如果说对于临界资源的操作耗时很长（特别是单个操作就大于1ms）的话，实际上性能上会有一定的问题，这也就是我们经常看到“的锁一直处于饥饿状态”的问题，对于这种情况，可能就需要另寻他法了。

好了，至此整个`sync.Mutex`的分析就此结束了，虽然只有短短200行代码（包括150行注释，实际代码估计就50行），但是其中的算法、设计的思想、编程的理念却是值得感悟，所谓大道至简、少即是多可能就是如此吧。


作者：PureWhite
链接：https://juejin.cn/post/6844903910541361160
来源：稀土掘金
著作权归作者所有。商业转载请联系作者获得授权，非商业转载请注明出处。


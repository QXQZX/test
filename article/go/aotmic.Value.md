---
title: 'Go语言之 atomic.Value如何不加锁保证数据线程安全？'
tags: ["Go"]
categories: ["Go"]
date: "2021-11-13T11:37:38+08:00"
toc: true
draft: false
---

很多人可能没有注意过，在 Go（甚至是大部分语言）中，一条普通的赋值语句其实不是一个原子操作。例如，在32位机器上写`int64`类型的变量就会有中间状态，它会被拆成两次写操作（汇编的`MOV`指令）——写低 32 位和写高 32 位。32机器上对int64进行赋值

如果一个线程刚写完低32位，还没来得及写高32位时，另一个线程读取了这个变量，那它得到的就是一个毫无逻辑的中间变量，这很有可能使我们的程序出现Bug。

这还只是一个基础类型，如果我们对一个结构体进行赋值，那它出现并发问题的概率就更高了。很可能写线程刚写完一小半的字段，读线程就来读取这个变量，那么就只能读到仅修改了一部分的值。这显然破坏了变量的完整性，读出来的值也是完全错误的。

面对这种多线程下变量的读写问题，`Go`给出的解决方案是`atomic.Value`，它使得我们可以不依赖于不保证兼容性的`unsafe.Pointer`类型，同时又能将任意数据类型的读写操作封装成原子性操作。

## atomic.Value的使用方式

`atomic.Value`类型对外提供了两个读写方法：
- `v.Store(c)` - 写操作，将原始的变量`c`存放到一个`atomic.Value`类型的`v`里。
- `c := v.Load()` - 读操作，从内存中线程安全的`v`中读取上一步存放的内容。

下面是一个简单的例子演示`atomic.Value`的用法。

```go
type Rectangle struct {
	length int
	width  int
}

var rect atomic.Value

func update(width, length int) {
	rectLocal := new(Rectangle)
	rectLocal.width = width
	rectLocal.length = length
	rect.Store(rectLocal)
}

func main() {
	wg := sync.WaitGroup{}
	wg.Add(10)
	// 10 个协程并发更新
	for i := 0; i < 10; i++ {
		go func(i int) {
			defer wg.Done()
			update(i, i+5)
		}(i)
	}
	wg.Wait()
	r := rect.Load().(*Rectangle)
	fmt.Printf("rect.width=%d\nrect.length=%d\n", r.width, r.length)
}

```

你可能会好奇，为什么`atomic.Value`在不加锁的情况下就提供了读写变量的线程安全保证，接下来我们就一起看看其内部实现。

## atomic.Value的内部实现

`atomic.Value`被设计用来存储任意类型的数据，所以它内部的字段是一个`interface{}`类型。

```go
// A Value provides an atomic load and store of a consistently typed value.
// The zero value for a Value returns nil from Load.
// Once Store has been called, a Value must not be copied.
//
// A Value must not be copied after first use.
type Value struct {
	v interface{}
}
```

除了`Value`外，`atomic`包内部定义了一个`ifaceWords`类型，这其实是`interface{}`的内部表示 (runtime.eface)，它的作用是将`interface{}`类型分解，得到其原始类型（typ）和真正的值（data）。

```go
// ifaceWords is interface{} internal representation.
type ifaceWords struct {
	typ  unsafe.Pointer
	data unsafe.Pointer
}
```



### 写入线程安全的保证

直接来看代码

```go
// Store sets the value of the Value to x.
// All calls to Store for a given Value must use values of the same concrete type.
// Store of an inconsistent type panics, as does Store(nil).
func (v *Value) Store(val interface{}) {
	if val == nil {
		panic("sync/atomic: store of nil value into Value")
	}
    
    // 通过unsafe.Pointer将现有的(v)和要写入的值(val) 分别转成ifaceWords类型。
    // 这样我们下一步就可以得到这两个interface{}的原始类型（typ）和真正的值（data）。
	vp := (*ifaceWords)(unsafe.Pointer(v))
	vlp := (*ifaceWords)(unsafe.Pointer(&val))
	for {
		typ := LoadPointer(&vp.typ)
		if typ == nil {
			// Attempt to start first store.
			// Disable preemption so that other goroutines can use
			// active spin wait to wait for completion; and so that
			// GC does not see the fake type accidentally.
			runtime_procPin()
			if !CompareAndSwapPointer(&vp.typ, nil, unsafe.Pointer(^uintptr(0))) {
				runtime_procUnpin()
				continue
			}
			// Complete first store.
			StorePointer(&vp.data, vlp.data)
			StorePointer(&vp.typ, vlp.typ)
			runtime_procUnpin()
			return
		}
		if uintptr(typ) == ^uintptr(0) {
			// First store in progress. Wait.
			// Since we disable preemption around the first store,
			// we can wait with active spinning.
			continue
		}
		// First store completed. Check type and overwrite data.
		if typ != vlp.typ {
			panic("sync/atomic: store of inconsistently typed value into Value")
		}
		StorePointer(&vp.data, vlp.data)
		return
	}
}
```

大概的逻辑：
- 开始就是一个无限 for 循环。配合`CompareAndSwap`使用，可以达到乐观锁的效果。
- 通过`LoadPointer`这个原子操作拿到当前`Value`中存储的类型。下面根据这个类型的不同，分3种情况处理。

1. 第一次写入

   一个`atomic.Value`实例被初始化后，它的`typ`和`data`字段会被设置为指针的零值 nil，所以先判断如果`typ`是否为nil，如果是那就证明这个`Value`实例还未被写入过数据。那之后就是一段初始写入的操作：

2. `runtime_procPin()`这是runtime中的一段函数，一方面它禁止了调度器对当前 goroutine 的抢占（preemption），使得它在执行当前逻辑的时候不被其他goroutine打断，以便可以尽快地完成工作。另一方面，在禁止抢占期间，GC 线程也无法被启用，这样可以防止 GC 线程看到一个莫名其妙的指向`^uintptr(0)`的类型（这是赋值过程中的中间状态）。
   
   1）使用`CAS`操作，先尝试将`typ`设置为`^uintptr(0)`这个中间状态。如果失败，则证明已经有别的线程抢先完成了赋值操作，那它就解除抢占锁，然后重新回到 for 循环第一步。

   2）如果设置成功，那证明当前线程抢到了这个"乐观锁”，它可以安全的把`v`设为传入的新值了。注意，这里是先写`data`字段，然后再写`typ`字段。**因为我们是以`typ`字段的值作为写入完成与否的判断依据的**

3. 第一次写入还未完成

   如果看到`typ`字段还是`^uintptr(0)`这个中间类型，证明刚刚的第一次写入还没有完成，所以它会继续循环，一直等到第一次写入完成。

4. 第一次写入已完成

   首先检查上一次写入的类型与这一次要写入的类型是否一致，如果不一致则抛出异常。反之，则直接把这一次要写入的值写入到`data`字段。

   

*这个逻辑的主要思想就是，为了完成多个字段的原子性写入，我们可以抓住其中的一个字段，以它的状态来标志整个原子写入的状态。*



### 读取（Load）操作

先上代码：

```go
// Load returns the value set by the most recent Store.
// It returns nil if there has been no call to Store for this Value.
func (v *Value) Load() (val interface{}) {
	vp := (*ifaceWords)(unsafe.Pointer(v))
	typ := LoadPointer(&vp.typ)
	if typ == nil || uintptr(typ) == ^uintptr(0) {
		// First store not yet completed.
		return nil
	}
	data := LoadPointer(&vp.data)
	vlp := (*ifaceWords)(unsafe.Pointer(&val))
	vlp.typ = typ
	vlp.data = data
	return
}
```

读取相对就简单很多了，它有两个分支：

1. 如果当前的`typ`是 nil 或者`^uintptr(0)`，那就证明第一次写入还没有开始，或者还没完成，那就直接返回 nil （不对外暴露中间状态）。
2. 否则，根据当前看到的`typ`和`data`构造出一个新的`interface{}`返回出去。



## 总结

本文由浅入深的介绍了`atomic.Value`的使用姿势，以及内部实现。另外，原子操作由**底层硬件**支持，对于一个变量更新的保护，原子操作通常会更有效率，并且更能利用计算机多核的优势，如果要更新的是一个复合对象，则应当使用`atomic.Value`封装好的实现。

而我们做并发同步控制常用到的`Mutex`锁，则是由操作系统的**调度器**实现，锁应当用来保护一段逻辑。

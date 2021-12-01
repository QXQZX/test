---
title: 'C++语言之 完美转发'
tags: ["C++"]
categories: ["C++"]
date: "2021-12-01T15:35:23+08:00"
toc: true
draft: false
---



完美转发 = 引用折叠 + 万能引用 + std::forward

<!--more-->



## 1. C++为什么需要完美转发？

```c++
template<typename T>
void print(T &t) {
    std::cout << "Lvalue ref" << std::endl;
}

template<typename T>
void print(T &&t) {
    std::cout << "Rvalue ref" << std::endl;
}

template<typename T>
void testForward(T &&v) {
    print(v); // v此时已经是个左值了,永远调用左值版本的print
    print(std::forward<T>(v)); // 转发，本文的重点
    print(std::move(v)); // move将左值转换为右值，永远调用右值版本的print

    std::cout << "======================" << std::endl;
}

int main(int argc, char *argv[]) {
    int x = 1;
    testForward(x); // 实参为左值
    testForward(std::move(x)); // 实参为右值
}
/*
Lvalue ref
Lvalue ref
Rvalue ref
======================
Lvalue ref
Rvalue ref
Rvalue ref
======================
*/
```

可以不难发现，无论传入的是左值还是右值，可以看到在testForward函数中，`T &&v`永远是个左值，所以直接`print(v)`一直会进入`void print(T &t)`函数。

而`std::move(v)`函数操作的作用时将`T &&v`这个左值转换为一个右值，所以`print(std::move(v));`一直会进入`void print(T &&t)`这个接收右值的函数。

但是我们期望，当`testForward(x)`传入左值的时候进入`void print(T &t)`函数；当 `testForward(std::move(x))`传入右值的时候进入`void print(T &&t)`函数。那怎么办呢？这就用到了`std::forward<T>()`转发操作，不难从打印结果中发现，此操作是符合预期的。



不难发现，本质问题在于，左值右值在函数调用时，都转化成了左值，使得函数转调用时无法判断左值和右值。



## 2. 引用折叠和万能引用

### 2.1 什么是引用折叠

> https://zhuanlan.zhihu.com/p/99524127
>
> 抽空总结下引用折叠

引用折叠只有两条规则:

- 一个 rvalue reference to an rvalue reference 会变成 (“折叠为”) 一个 rvalue reference.
- 所有其他种类的"引用的引用" (i.e., 组合当中含有lvalue reference) 都会折叠为 lvalue reference.

### 2.2 什么是万能引用

这个问题的本质实际上是，类型声明当中的“`&&`”有的时候意味着 rvalue reference，但有的时候意味着 rvalue reference *或者* lvalue reference。因此，源代码当中出现的 “`&&`” 有可能是 “`&`” 的意思，即是说，语法上看着像 rvalue reference (“`&&`”)，但实际上却代表着一个lvalue reference (“`&`”)。在这种情况下，此种引用比 lvalue references 或者 rvalue references 都要来的更灵活。

Rvalue references 只能绑定到右值上，lvalue references 除了可以绑定到左值上，在**某些条件**下还可以绑定到右值上。这里某些条件绑定右值为：常左值引用绑定到右值，非常左值引用不可绑定到右值！

例如：

```cpp
string &s = "asd";  // error
const string &s = "asd";  // ok
```

规则简化如下：

```text
左值引用   {左值}  
右值引用   {右值}
常左值引用  {右值}
```

相比之下，声明中带 “`&&`” 的，可能是lvalue references 或者 rvalue references 的引用可以绑定到任何东西上。这种引用灵活也忒灵活了，值得单独给它们起个名字。我称它们为 *universal references*(万能引用或转发引用、通用引用)。

拓展：在资料[6]中提到了const的重要性!

例如：

```cpp
string f() { return "abc"; }

void g() {
    const string &s = f(); // still legal?
    cout << s << endl;
}
```

上面g函数中合法？

答案是合法的，原因是s是个左值，类型是常左值引用，而f()是个右值，前面提到常左值引用可以绑定到右值！所以合法，当然把`const`去掉，便是不合法！



到底 “`&&`” 什么时候才意味着一个universal reference呢(即，代码当中的“`&&`”实际上可能是 “`&`”)，具体细节还挺棘手的，所以这些细节我推迟到后面再讲。现在，我们还是先集中精力研究下下面的经验原则，因为你在日常的编程工作当中需要牢记它：

> If a variable or parameter is declared to have type **T&&** for some **deduced type** `T`, that variable or parameter is a *universal reference*.
> 如果一个变量或者参数被声明为**T&&**，其中T是**被推导**的类型，那这个变量或者参数就是一个*universal reference*。

"T需要是一个被推导类型"这个要求限制了universal references的出现范围。必须具有形如`T&&`。



出现的场景

* 在实践当中，几乎所有的universal references都是函数模板的参数。因为`auto`声明的变量的类型推导规则本质上和模板是一样的，所以使用auto的时候你也可能得到一个universal references。

* 使用typedef和decltype的时候也可能会出现universal references，但在我们讲解这些繁琐的细节之前，我们可以暂时认为universal references只会出现在模板参数和由auto声明的变量当中。



和所有的引用一样，你必须对universal references进行初始化，而且正是universal reference的initializer决定了它到底代表的是lvalue reference 还是 rvalue reference:

- 如果用来初始化universal reference的表达式是一个左值，那么universal reference就变成lvalue reference。
- 如果用来初始化universal reference的表达式是一个右值，那么universal reference就变成rvalue reference。

上述可以根据下面代码例子理解：或者上面例子中的`void testForward(T &&v)`既可以接收左值也可以接收右值

```cpp
template<typename T>
void f(T&& param); 

int main() {
	int a;
    f(a);   // 传入左值,那么上述的T&& 就是lvalue reference,也就是左值引用绑定到了左值
    f(1);   // 传入右值,那么上述的T&& 就是rvalue reference,也就是右值引用绑定到了左值   
}
```



## 3. std::forward原理

std::forward不是独自运作的，完美转发 = std::forward + 万能引用 + 引用折叠。三者合一才能实现完美转发的效果。

std::forward的正确运作的前提，是引用折叠机制，为T &&类型的万能引用中的模板参数T赋了一个恰到好处的值。我们用T去指明std::forward的模板参数，从而使得std::forward返回的是正确的类型。



### 3.1 testForward(x)

回到上面的例子。先考虑`testForward(x);`这一行代码。

#### 3.1.1 实例化testForward

根据万能引用的实例化规则，实例化的testForward大概长这样：

```cpp
T = int &
void testForward(int & && v){
    print(std::forward<T>(v));
}
```

又根据引用折叠，上面的等价于下面的代码：

```cpp
T = int &
void testForward(int & v){
    print(std::forward<int &>(v));
}
```

如果你正确的理解了引用折叠，那么这一步是很好理解的。



#### 3.1.2 实例化std::forward

> 注：C++ Primer：forward必须通过显式模板实参来调用，不能依赖函数模板参数推导。

接下来我们来看下`std::forward`在libstdc++中的实现：

```cpp
68   /**
69    *  @brief  Forward an lvalue.
70    *  @return The parameter cast to the specified type.
71    *
72    *  This function is used to implement "perfect forwarding".
73    */
74   template<typename _Tp>
75     constexpr _Tp&&
76     forward(typename std::remove_reference<_Tp>::type& __t) noexcept
77     { return static_cast<_Tp&&>(__t); }
```

由于Step1中我们调用`std::forward<int &>`，所以此处我们代入`T = int &`，我们有：

```cpp
constexpr int & && //折叠
forward(typename std::remove_reference<int &>::type& __t) noexcept //remove_reference的作用与名字一致，不过多解释
 { return static_cast<int & &&>(__t); } //折叠
```

这里又发生了2次引用折叠，所以上面的代码等价于：

```cpp
constexpr int & //折叠
forward(int & __t) noexcept //remove_reference的作用与名字一致，不过多解释
 { return static_cast<int &>(__t); } //折叠
```

所以最终`std::forward<int &>(v)`的作用就是将参数强制转型成`int &`，而`int &`为左值。所以，调用左值版本的print。



### 3.2 testForward(std::move(x))

接下来，考虑`testForward(std::move(x))`的情况。

#### 3.2.1 实例化testForward

`testForward(std::move(x))`也就是`testForward(static_cast<int &&>(x))`。根据万能引用的实例化规则，实例化的testForward大概长这样：

```cpp
T = int 
void testForward(int && v){
    print(std::forward<int>(v));
}
```


万能引用绑定到右值上时，不会发生引用折叠，所以这里没有引用折叠。



#### 3.2.2 实例化std::forward

> 注：C++ Primer：forward必须通过显式模板实参来调用，不能依赖函数模板参数推导。

这里用到的std::forward的代码和上面的一样，故略去。

由于Step1中我们调用`std::forward<int>`，所以此处我们代入`T = int`，我们有：

```cpp
constexpr int && 
forward(typename std::remove_reference<int>::type& __t) noexcept //remove_reference的作用与名字一致，不过多解释
 { return static_cast<int &&>(__t); }
```

这里又发生了2次引用折叠，所以上面的代码等价于：

```cpp
constexpr int &&
forward(int & __t) noexcept //remove_reference的作用与名字一致，不过多解释
 { return static_cast<int &&>(__t); } 
```

所以最终

```
std::forward<int>(v)
```

的作用就是将参数强制转型成`int &&`，为右值。所以，调用右值版本的print。



参考

* https://lamforest.github.io/2021/04/29/cpp/wan-mei-zhuan-fa-yin-yong-zhe-die-wan-neng-yin-yong-std-forward/

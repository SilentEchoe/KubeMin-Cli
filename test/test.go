/*package  main

import (
	"fmt"
	"sync"
)

var (
	counter int
	wg sync.WaitGroup

	mutex sync.Mutex
)

func main()  {
	wg.Add(2)

	go incCounter(1)
	go incCounter(2)

	// 等待goroutine 结束
	wg.Wait()

	fmt.Println("Final counter : %d\\n", counter)
}

func  incCounter(id int)  {
	defer wg.Done()

	for count := 0;count < 2 ; count++ {
		// 和C#的Lock 锁一样 锁住共享资源 只允许一个访问
		mutex.Lock()
		{
			value := counter

			//runtime.Gosched()

			value ++

			counter = value
		}
		// 释放锁
		mutex.Unlock()
	}

}

*/


package  main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

var wg sync.WaitGroup

func init()  {
	rand.Seed(time.Now().UnixNano())
}

func main()  {
	baton := make(chan int)

	wg.Add(1)

	go Runner(baton)

	baton <- 1

	wg.Wait()
}

func Runner(baton chan int){
	var newRunner int

	// 将通道的值传入
	runner := <- baton

	fmt.Println("Runner %d Running With Baton\n", runner)

	if runner  != 4 {
		newRunner = runner + 1
		fmt.Printf("Runner %d To The Line\n", newRunner)
		// 递归
		go Runner(baton)
	}
	time.Sleep(100 * time.Millisecond)

	// 判断是否跑了一圈（默认一圈是四百米）
	if runner == 4 {
		fmt.Printf("Runner %d Finished, Race Over\n", runner)
		wg.Done()
		return
	}

	fmt.Printf("Runner %d Exchange With Runner %d\n",
		 runner,
		newRunner)
	// 讲新的值传给通道
	baton <- newRunner

}



package  main

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


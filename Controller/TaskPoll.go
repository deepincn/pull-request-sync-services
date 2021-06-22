package Controller

import (
	"github.com/sirupsen/logrus"
	"sync"
	"time"
)

// 定义任务接口,所有实现该接口的均实现工作池
type Task interface {
	Name() string
	DoTask() error
}

// 定义工作结构体
type Job struct {
	Task Task
}

// 定义工作者
type Worker struct {
	WorkerPool chan chan Job // 工人对象池
	JobChannel chan Job      // 管道里面拿Job
	quit       chan bool
	sender     *Sender
}

// 新建一个工作者
func NewWorker(workerPool chan chan Job) Worker {
	return Worker{
		WorkerPool: workerPool,     // 工人对象池
		JobChannel: make(chan Job), //工人的任务
		quit:       make(chan bool),
	}
}

// 工作池启动主函数
func (w *Worker) Start() {
	// 开一个新的协程
	go func() {
		for {
			// 注册任务到工作池
			w.WorkerPool <- w.JobChannel
			select {
			// 接收到任务
			case job := <-w.JobChannel:
				// 执行任务
				// 这里查找一下项目对应的锁，目的是按队列处理
				w.sender.TaskMutex.Lock()
				mutex, ok := w.sender.TaskMutexMap[job.Task.Name()]

				if ok {
					logrus.Debug("found running task")
				}
				if !ok {
					logrus.Debug("not mutex, add a new mutex")
					mutex = &sync.Mutex{}
					w.sender.TaskMutexMap[job.Task.Name()] = mutex
				}

				// TODO: 性能问题
				go func() {
					time.Sleep(1 * time.Second)
					w.sender.TaskMutex.Unlock()
				}()

				logrus.Debugf("task %v, try locking...", job.Task.Name())
				mutex.Lock()
				logrus.Debugf("task %v is locking...", job.Task.Name())

				err := job.Task.DoTask()
				if err != nil {
					logrus.Errorf("task %v failed.", job.Task.Name())
				}

				mutex.Unlock()
				logrus.Debug("task finished, unlocking...")
			// 接收退出的任务, 停止任务
			case <-w.quit:
				return
			}
		}
	}()
}

// 退出执行工作
func (w *Worker) Stop() {
	go func() {
		w.quit <- true
	}()
}

// 定义任务发送者
type Sender struct {
	maxWorkers   int           // 最大工人数
	WorkerPool   chan chan Job // 注册工作通道
	quit         chan bool     // 退出信号
	TaskMutex    *sync.Mutex
	TaskMutexMap map[string]*sync.Mutex //任务队列的锁
}

// 注册新发送者
func NewSender(maxWorkers int) *Sender {
	Pool := make(chan chan Job, maxWorkers)
	return &Sender{
		WorkerPool:   Pool,       // 将工作者放到一个工作池中
		maxWorkers:   maxWorkers, // 最大工作者数量
		quit:         make(chan bool),
		TaskMutex:    &sync.Mutex{},
		TaskMutexMap: make(map[string]*sync.Mutex), // 初始化task的锁，每个仓库的pull request需要按队列执行
	}
}

// 工作分发器
func (s *Sender) Run(queue *chan Job) {
	for i := 0; i < s.maxWorkers; i++ {
		worker := NewWorker(s.WorkerPool)
		// use mutex
		worker.sender = s
		// 执行任务
		worker.Start()
	}
	// 监控任务发送
	go s.Send(queue)
}

// 退出发放工作
func (s *Sender) Quit() {
	go func() {
		s.quit <- true
	}()
}

func (s *Sender) Send(queue *chan Job) {
	for {
		select {
		// 接收到任务
		case job := <-*queue:
			go func(job Job) {
				jobChan := <-s.WorkerPool
				jobChan <- job
			}(job)
		// 退出任务分发
		case <-s.quit:
			return
		}
	}
}

// 初始化对象池，返回任务队列管道
func InitPool(maxWorkers int, maxQueue int) *chan Job {
	// 初始化一个任务发送者,指定工作者数量
	send := NewSender(maxWorkers)
	// 定义全部的工作队列
	JobQueue := make(chan Job, maxQueue)
	// 一直运行任务发送
	send.Run(&JobQueue)

	// TODO: 这里需要设计成调用返回新的协程池
	return &JobQueue
}

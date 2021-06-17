package github

import (
	"github.com/colorful-fullstack/PRTools/config"
	"github.com/colorful-fullstack/PRTools/database"
	"github.com/google/go-github/v35/github"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"sync"
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

// 定义全部的工作队列
var JobQueue chan Job

// 全局task锁，用于检查task
var TaskMutex sync.Mutex

// 定义task的锁
var TaskMutexMap map[string]sync.Mutex

// 定义工作者
type Worker struct {
	WorkerPool chan chan Job // 工人对象池
	JobChannel chan Job      // 管道里面拿Job
	quit       chan bool
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
				TaskMutex.Lock()
				mutex, ok := TaskMutexMap[job.Task.Name()]
				if ok {
					logrus.Debug("found running task")
				}
				if !ok {
					logrus.Debug("not mutex, add a new mutex")
					mutex = sync.Mutex{}
					TaskMutexMap[job.Task.Name()] = mutex
				}
				logrus.Debugf("task %v, try locking...", job.Task.Name())
				mutex.Lock()
				logrus.Debugf("task %v is locking...", job.Task.Name())
				TaskMutex.Unlock()

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
	maxWorkers int           // 最大工人数
	WorkerPool chan chan Job // 注册工作通道
	quit       chan bool     // 退出信号
}

// 注册新发送者
func NewSender(maxWorkers int) *Sender {
	Pool := make(chan chan Job, maxWorkers)
	return &Sender{
		WorkerPool: Pool,       // 将工作者放到一个工作池中
		maxWorkers: maxWorkers, // 最大工作者数量
		quit:       make(chan bool),
	}
}

// 工作分发器
func (s *Sender) Run() {
	for i := 0; i < s.maxWorkers; i++ {
		worker := NewWorker(s.WorkerPool)
		// 执行任务
		worker.Start()
	}
	// 监控任务发送
	go s.Send()
}

// 退出发放工作
func (s *Sender) Quit() {
	go func() {
		s.quit <- true
	}()
}

func (s *Sender) Send() {
	for {
		select {
		// 接收到任务
		case job := <-JobQueue:
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

// 初始化对象池
func InitPool() {
	maxWorkers := 5
	maxQueue := 20
	// 初始化一个任务发送者,指定工作者数量
	send := NewSender(maxWorkers)
	// 指定任务的队列长度
	JobQueue = make(chan Job, maxQueue)
	// 初始化task的锁，每个仓库的pull request需要按队列执行
	TaskMutexMap = make(map[string]sync.Mutex)
	// 一直运行任务发送
	send.Run()
}

type PRTask struct {
	event   *github.PullRequestEvent
	manager *Manager
}

func (t *PRTask) Name() string {
	return t.event.GetRepo().GetName()
}

func (t *PRTask) DoTask() error {
	t.pullRequestHandler(t.event)
	return nil
}

// Manager is github module manager
type Manager struct {
	conf *config.Yaml
	db   *database.DataBase
}

// New creates
func New(conf *config.Yaml, db *database.DataBase) *Manager {
	InitPool()
	return &Manager{
		conf: conf,
		db:   db,
	}
}

// WebhookHandle init
func (m *Manager) WebhookHandle(rw http.ResponseWriter, r *http.Request) {
	var event interface{}

	payload, err := github.ValidatePayload(r, []byte(""))
	if err != nil {
		logrus.Errorf("validate payload failed: %v", err)
		return
	}

	event, err = github.ParseWebHook(github.WebHookType(r), payload)
	if err != nil {
		logrus.Errorf("parse webhook failed: %v", err)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		logrus.Errorf("request body: %v", string(body))

		rw.WriteHeader(400)
		result, err := rw.Write([]byte(err.Error()))
		if err != nil {
			logrus.Errorf("rw write: %v", result)
		}
		return
	}

	switch event := event.(type) {
	case *github.IssueEvent:
		logrus.Infof("IssueEvent: %v", *event.ID)
		break
	case *github.PullRequestEvent:
		go func() {
			logrus.Infof("PullRequestEvent: %v", *event.Number)
			task := &PRTask{
				event:   event,
				manager: m,
			}
			JobQueue <- Job{
				Task: task,
			}
		}()
		break
	case *github.PushEvent:
		logrus.Infof("PushEvent: %v", *event.PushID)
		break
	}
}

package stack

import (
	"fmt"
	"sync"
)

var (
	UniqueTaskManager = &taskManager{
		tasks: make(map[string]interface{}),
	}
)

func GenerateCatalogTaskID(deviceID string) string {
	return fmt.Sprintf("%s_catalog", deviceID)
}

type taskManager struct {
	lock  sync.Mutex
	tasks map[string]interface{}
}

func (t *taskManager) Commit(id string, task func(), data interface{}) bool {
	t.lock.Lock()
	defer t.lock.Unlock()

	if _, ok := t.tasks[id]; ok {
		return false
	}

	t.tasks[id] = data

	go func() {
		task()
		t.lock.Lock()
		defer t.lock.Unlock()
		delete(t.tasks, id)
	}()

	return true
}

func (t *taskManager) Find(id string) interface{} {
	t.lock.Lock()
	defer t.lock.Unlock()

	if data, ok := t.tasks[id]; ok {
		return data
	}
	return nil
}

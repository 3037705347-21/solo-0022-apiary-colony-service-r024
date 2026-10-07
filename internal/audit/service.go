// Package audit 提供审计事件的读取入口。事件由仓储在业务写入被采纳时
// 同源追加，只增不改；本包只负责按蜂场或蜂箱读出已经发生的事实。
package audit

import (
	"example.com/apiary-colony-service/internal/domain"
	"example.com/apiary-colony-service/internal/store"
)

// Service 按蜂场或蜂箱读取审计事件。
type Service struct {
	store *store.Memory
}

func New(memory *store.Memory) *Service {
	return &Service{store: memory}
}

// ListByApiary 返回蜂场及其全部蜂箱的事件，按顺序号升序；蜂场不存在返回未找到。
func (s *Service) ListByApiary(apiaryID string) ([]domain.Event, error) {
	return s.store.EventsByApiary(apiaryID)
}

// ListByHive 返回单个蜂箱的事件，按顺序号升序；蜂箱不存在返回未找到。
func (s *Service) ListByHive(hiveID string) ([]domain.Event, error) {
	return s.store.EventsByHive(hiveID)
}

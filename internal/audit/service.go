package audit

import (
	"example.com/apiary-colony-service/internal/domain"
	"example.com/apiary-colony-service/internal/store"
)

// Service 提供按蜂场、按蜂箱读取审计事件的能力。
// 事件只追加不可改，读取始终得到按全局序号升序的稳定结果。
type Service struct {
	store *store.Memory
}

func New(memory *store.Memory) *Service {
	return &Service{store: memory}
}

// ByApiary 返回某座蜂场范围内的审计事件：包含蜂场录入以及其下全部蜂箱的
// 安装、状态推进、巡检与换王事件，按事件发生顺序（全局序号）升序。
func (s *Service) ByApiary(apiaryID string) ([]domain.AuditEvent, error) {
	return s.store.ApiaryEvents(apiaryID)
}

// ByHive 返回某只蜂箱范围内的审计事件：包含该蜂箱的安装、状态推进、
// 巡检与换王事件，按事件发生顺序（全局序号）升序。
func (s *Service) ByHive(hiveID string) ([]domain.AuditEvent, error) {
	return s.store.HiveEvents(hiveID)
}

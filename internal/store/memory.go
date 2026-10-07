package store

import (
	"fmt"
	"sync"
	"time"

	"example.com/apiary-colony-service/internal/domain"
)

// Memory 是并发安全的内存仓储。所有写入都在写锁内完成状态校验，
// 读取则通过 Snapshot 得到一致视图。
//
// 审计事件与业务结果在同一把写锁内一起提交：任何校验未通过或冲突的请求
// 都会在追加事件之前返回，因此被拒绝的写入绝不会留下审计事件。
type Memory struct {
	mu sync.RWMutex

	apiaries    map[string]domain.Apiary
	hives       map[string]domain.Hive
	inspections map[string][]domain.Inspection
	queens      map[string][]domain.Queen

	// events 仅按全局序号追加，append 顺序即读取顺序，保证稳定排序。
	events []domain.AuditEvent

	inspectionSeq int
	queenSeq      int
	eventSeq      int64
}

func NewMemory() *Memory {
	return &Memory{
		apiaries:    make(map[string]domain.Apiary),
		hives:       make(map[string]domain.Hive),
		inspections: make(map[string][]domain.Inspection),
		queens:      make(map[string][]domain.Queen),
	}
}

func (m *Memory) CreateApiary(apiary domain.Apiary, requestID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.apiaries[apiary.ID]; ok {
		return domain.Conflict("蜂场 %q 已存在", apiary.ID)
	}
	event, err := domain.NewAuditEvent(
		m.nextEventSeqLocked(),
		domain.AuditApiaryRegistered,
		apiary.ID, "", requestID, apiary.CreatedAt,
		domain.ApiaryRegisteredDetail{
			Name:     apiary.Name,
			District: apiary.District,
			Keeper:   apiary.Keeper,
		},
	)
	if err != nil {
		return err
	}
	m.apiaries[apiary.ID] = apiary
	m.events = append(m.events, event)
	return nil
}

func (m *Memory) GetApiary(id string) (domain.Apiary, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	apiary, ok := m.apiaries[id]
	return apiary, ok
}

func (m *Memory) CreateHive(hive domain.Hive, requestID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.apiaries[hive.ApiaryID]; !ok {
		return domain.NotFound("蜂场", hive.ApiaryID)
	}
	if _, ok := m.hives[hive.ID]; ok {
		return domain.Conflict("蜂箱 %q 已存在", hive.ID)
	}
	event, err := domain.NewAuditEvent(
		m.nextEventSeqLocked(),
		domain.AuditHiveInstalled,
		hive.ApiaryID, hive.ID, requestID, hive.InstalledAt,
		domain.HiveInstalledDetail{
			Model:    hive.Model,
			NewState: hive.State,
		},
	)
	if err != nil {
		return err
	}
	m.hives[hive.ID] = hive
	m.events = append(m.events, event)
	return nil
}

func (m *Memory) GetHive(id string) (domain.Hive, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	hive, ok := m.hives[id]
	return hive, ok
}

// AdvanceHive 在同一把写锁内完成状态读取、迁移校验与写入，避免并发迁移产生交错结果。
// 迁移被拒绝时（未知蜂箱、迁移到自身、非法迁移）不追加任何事件。
func (m *Memory) AdvanceHive(hiveID string, target domain.HiveState, now time.Time, requestID string) (domain.Transition, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	hive, ok := m.hives[hiveID]
	if !ok {
		return domain.Transition{}, domain.NotFound("蜂箱", hiveID)
	}
	if hive.State == target {
		return domain.Transition{}, domain.InvalidState("蜂箱 %q 已经处于状态 %q", hiveID, target)
	}
	if !domain.CanTransitionHive(hive.State, target) {
		return domain.Transition{}, domain.InvalidState(
			"蜂箱 %q 不允许从 %q 迁移到 %q", hiveID, hive.State, target)
	}
	from := hive.State
	hive.State = target
	event, err := domain.NewAuditEvent(
		m.nextEventSeqLocked(),
		domain.AuditHiveStateAdvanced,
		hive.ApiaryID, hiveID, requestID, now,
		domain.HiveStateAdvancedDetail{From: from, To: target},
	)
	if err != nil {
		return domain.Transition{}, err
	}
	m.hives[hiveID] = hive
	m.events = append(m.events, event)
	return domain.Transition{HiveID: hiveID, From: from, To: target}, nil
}

// AddInspection 分配巡检编号、写入记录，并推进蜂箱的最近巡检时间。
// 蜂箱存在与可接受巡检的判定与写入处于同一把锁，避免退役与巡检之间的竞态。
// 蜂箱未知或已退役时直接返回，不追加巡检事件。
func (m *Memory) AddInspection(hiveID string, inspection domain.Inspection, now time.Time, requestID string) (domain.Inspection, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	hive, ok := m.hives[hiveID]
	if !ok {
		return domain.Inspection{}, domain.NotFound("蜂箱", hiveID)
	}
	if !hive.AcceptsInspection() {
		return domain.Inspection{}, domain.InvalidState("蜂箱 %q 处于 %q 状态，不接受巡检", hiveID, hive.State)
	}
	m.inspectionSeq++
	inspection.ID = fmt.Sprintf("inspection-%06d", m.inspectionSeq)
	inspection.HiveID = hiveID
	event, err := domain.NewAuditEvent(
		m.nextEventSeqLocked(),
		domain.AuditInspectionRecorded,
		hive.ApiaryID, hiveID, requestID, now,
		domain.InspectionRecordedDetail{
			InspectionID:   inspection.ID,
			ObservedAt:     inspection.ObservedAt,
			ColonyStrength: inspection.ColonyStrength,
			BroodPattern:   inspection.BroodPattern,
			Temper:         inspection.Temper,
			MiteCount:      inspection.MiteCount,
			Note:           inspection.Note,
			Concerns:       append([]string{}, inspection.Concerns...),
		},
	)
	if err != nil {
		return domain.Inspection{}, err
	}
	m.inspections[hiveID] = append(m.inspections[hiveID], inspection)

	observedAt := inspection.ObservedAt
	hive.LastInspectionAt = &observedAt
	m.hives[hiveID] = hive
	m.events = append(m.events, event)
	return inspection, nil
}

// AddQueen 关闭原有产卵蜂王并写入新蜂王，两件事在同一把锁内完成，
// 保证任意时刻同一蜂箱至多只有一只 laying 蜂王。
// 全部校验通过、关闭旧记录与新记录编号分配完成后才提交并追加一条换王事件；
// 蜂箱未知、已退役或旧蜂王已关闭等情况下不产生任何变更，也不留下事件。
func (m *Memory) AddQueen(
	hiveID string,
	queen domain.Queen,
	closeState domain.QueenState,
	reason string,
	now time.Time,
	requestID string,
) (domain.Queen, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	hive, ok := m.hives[hiveID]
	if !ok {
		return domain.Queen{}, domain.NotFound("蜂箱", hiveID)
	}
	if !hive.AcceptsQueenChange() {
		return domain.Queen{}, domain.InvalidState("蜂箱 %q 处于 %q 状态，不接受换王", hiveID, hive.State)
	}
	previousQueenID := hive.ActiveQueenID
	if previousQueenID != "" {
		if err := m.canCloseQueenLocked(hiveID, previousQueenID); err != nil {
			return domain.Queen{}, err
		}
	}
	m.queenSeq++
	queen.ID = fmt.Sprintf("queen-%06d", m.queenSeq)
	queen.HiveID = hiveID
	queen.State = domain.QueenLaying
	detail := domain.QueenReplacedDetail{
		NewQueenID:   queen.ID,
		Origin:       queen.Origin,
		IntroducedAt: queen.IntroducedAt,
		MarkColor:    queen.MarkColor,
		Reason:       reason,
	}
	if previousQueenID != "" {
		// 只有真正关闭旧蜂王时才记录前序编号与状态，首次引入不带前序事实。
		detail.PreviousQueenID = previousQueenID
		detail.PreviousState = closeState
	}
	event, err := domain.NewAuditEvent(
		m.nextEventSeqLocked(),
		domain.AuditQueenReplaced,
		hive.ApiaryID, hiveID, requestID, now,
		detail,
	)
	if err != nil {
		return domain.Queen{}, err
	}
	if previousQueenID != "" {
		m.closeQueenLocked(hiveID, previousQueenID, closeState, now)
	}
	m.queens[hiveID] = append(m.queens[hiveID], queen)

	hive.ActiveQueenID = queen.ID
	m.hives[hiveID] = hive
	m.events = append(m.events, event)
	return queen, nil
}

// canCloseQueenLocked 只校验目标蜂王当前仍处于 laying，不产生任何变更。
func (m *Memory) canCloseQueenLocked(hiveID, queenID string) error {
	for _, item := range m.queens[hiveID] {
		if item.ID == queenID {
			if !item.IsLaying() {
				return domain.Conflict("蜂王 %q 已经关闭", queenID)
			}
			return nil
		}
	}
	return domain.Storage("蜂箱 %q 上的蜂王 %q 记录缺失", hiveID, queenID)
}

func (m *Memory) closeQueenLocked(hiveID, queenID string, state domain.QueenState, now time.Time) {
	items := m.queens[hiveID]
	for index, item := range items {
		if item.ID != queenID {
			continue
		}
		closedAt := now.UTC()
		item.State = state
		item.ClosedAt = &closedAt
		items[index] = item
		m.queens[hiveID] = items
		return
	}
}

func (m *Memory) ListInspections(hiveID string) ([]domain.Inspection, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, ok := m.hives[hiveID]; !ok {
		return nil, domain.NotFound("蜂箱", hiveID)
	}
	items := m.inspections[hiveID]
	out := make([]domain.Inspection, len(items))
	copy(out, items)
	return out, nil
}

func (m *Memory) ListQueens(hiveID string) ([]domain.Queen, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, ok := m.hives[hiveID]; !ok {
		return nil, domain.NotFound("蜂箱", hiveID)
	}
	items := m.queens[hiveID]
	out := make([]domain.Queen, len(items))
	copy(out, items)
	return out, nil
}

// ApiaryEvents 返回蜂场范围内与已采纳写入同源的审计事件，按全局序号升序。
// 蜂场不存在时返回未找到；事件内容为深拷贝，调用方无法改动既有事件。
func (m *Memory) ApiaryEvents(apiaryID string) ([]domain.AuditEvent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, ok := m.apiaries[apiaryID]; !ok {
		return nil, domain.NotFound("蜂场", apiaryID)
	}
	return m.eventsForLocked(func(event domain.AuditEvent) bool {
		return event.ApiaryID == apiaryID
	}), nil
}

// HiveEvents 返回蜂箱范围内与已采纳写入同源的审计事件，按全局序号升序。
// 蜂箱不存在时返回未找到；事件内容为深拷贝，调用方无法改动既有事件。
func (m *Memory) HiveEvents(hiveID string) ([]domain.AuditEvent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, ok := m.hives[hiveID]; !ok {
		return nil, domain.NotFound("蜂箱", hiveID)
	}
	return m.eventsForLocked(func(event domain.AuditEvent) bool {
		return event.HiveID == hiveID
	}), nil
}

func (m *Memory) eventsForLocked(match func(domain.AuditEvent) bool) []domain.AuditEvent {
	out := make([]domain.AuditEvent, 0)
	for _, event := range m.events {
		if !match(event) {
			continue
		}
		copied := event
		copied.Detail = append([]byte(nil), event.Detail...)
		out = append(out, copied)
	}
	return out
}

// nextEventSeqLocked 在写锁内生成下一个全局事件序号。所有事件按序号
// 升序追加，因此同一范围重复读取得到的顺序与内容保持稳定。
func (m *Memory) nextEventSeqLocked() int64 {
	m.eventSeq++
	return m.eventSeq
}

// Snapshot 返回深拷贝的一致视图，供汇总类读取使用。
func (m *Memory) Snapshot() Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	snapshot := Snapshot{
		Apiaries:          make(map[string]domain.Apiary, len(m.apiaries)),
		Hives:             make(map[string]domain.Hive, len(m.hives)),
		InspectionsByHive: make(map[string][]domain.Inspection, len(m.inspections)),
		QueensByHive:      make(map[string][]domain.Queen, len(m.queens)),
	}
	for id, apiary := range m.apiaries {
		snapshot.Apiaries[id] = apiary
	}
	for id, hive := range m.hives {
		snapshot.Hives[id] = cloneHive(hive)
	}
	for id, items := range m.inspections {
		copied := make([]domain.Inspection, len(items))
		copy(copied, items)
		for index := range copied {
			copied[index].Concerns = append([]string{}, items[index].Concerns...)
		}
		snapshot.InspectionsByHive[id] = copied
	}
	for id, items := range m.queens {
		copied := make([]domain.Queen, len(items))
		copy(copied, items)
		for index := range copied {
			if items[index].ClosedAt != nil {
				closedAt := *items[index].ClosedAt
				copied[index].ClosedAt = &closedAt
			}
		}
		snapshot.QueensByHive[id] = copied
	}
	return snapshot
}

func cloneHive(hive domain.Hive) domain.Hive {
	if hive.LastInspectionAt != nil {
		observedAt := *hive.LastInspectionAt
		hive.LastInspectionAt = &observedAt
	}
	return hive
}

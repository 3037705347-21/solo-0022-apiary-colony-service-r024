package domain

import (
	"encoding/json"
	"fmt"
	"time"
)

// AuditEventType 标识一次被采纳写入所对应的审计事实类型。
type AuditEventType string

const (
	AuditApiaryRegistered   AuditEventType = "apiary_registered"
	AuditHiveInstalled      AuditEventType = "hive_installed"
	AuditHiveStateAdvanced  AuditEventType = "hive_state_advanced"
	AuditInspectionRecorded AuditEventType = "inspection_recorded"
	AuditQueenReplaced      AuditEventType = "queen_replaced"
)

// AuditEvent 是一次成功写入的不可变事实。事件只追加、不更新、不删除，
// 与产生它的业务结果在同一把写锁内提交，因此读取到的事件一定对应已采纳的结果。
type AuditEvent struct {
	Seq        int64           `json:"seq"`
	ID         string          `json:"id"`
	Type       AuditEventType  `json:"type"`
	ApiaryID   string          `json:"apiaryId"`
	HiveID     string          `json:"hiveId,omitempty"`
	RequestID  string          `json:"requestId"`
	OccurredAt time.Time       `json:"occurredAt"`
	Detail     json.RawMessage `json:"detail"`
}

// ApiaryRegisteredDetail 是录入蜂场事件的事实明细。
type ApiaryRegisteredDetail struct {
	Name     string `json:"name"`
	District string `json:"district"`
	Keeper   string `json:"keeper"`
}

// HiveInstalledDetail 是安装蜂箱事件的事实明细。
type HiveInstalledDetail struct {
	Model    HiveModel `json:"model"`
	NewState HiveState `json:"newState"`
}

// HiveStateAdvancedDetail 是蜂群状态推进事件的事实明细，只记录实际生效的迁移。
type HiveStateAdvancedDetail struct {
	From HiveState `json:"from"`
	To   HiveState `json:"to"`
}

// InspectionRecordedDetail 是巡检提交事件的事实明细，关注项与保存的巡检记录同源派生。
type InspectionRecordedDetail struct {
	InspectionID   string       `json:"inspectionId"`
	ObservedAt     time.Time    `json:"observedAt"`
	ColonyStrength int          `json:"colonyStrength"`
	BroodPattern   BroodPattern `json:"broodPattern"`
	Temper         Temper       `json:"temper"`
	MiteCount      int          `json:"miteCount"`
	Note           string       `json:"note"`
	Concerns       []string     `json:"concerns"`
}

// QueenReplacedDetail 是换王事件的事实明细；首次引入蜂王时 Previous 字段缺省。
type QueenReplacedDetail struct {
	PreviousQueenID string      `json:"previousQueenId,omitempty"`
	PreviousState   QueenState  `json:"previousState,omitempty"`
	NewQueenID      string      `json:"newQueenId"`
	Origin          QueenOrigin `json:"origin"`
	IntroducedAt    time.Time   `json:"introducedAt"`
	MarkColor       string      `json:"markColor"`
	Reason          string      `json:"reason"`
}

// AuditEventID 由全局序号生成稳定的事件编号。
func AuditEventID(seq int64) string {
	return fmt.Sprintf("event-%08d", seq)
}

// NewAuditEvent 固化一条审计事件。明细在构造时序列化，事件一旦生成即不可再改。
func NewAuditEvent(
	seq int64,
	eventType AuditEventType,
	apiaryID, hiveID, requestID string,
	occurredAt time.Time,
	detail any,
) (AuditEvent, error) {
	if requestID == "" {
		return AuditEvent{}, Invalid("requestId", "审计事件必须携带请求编号")
	}
	if !validAuditEventType(eventType) {
		return AuditEvent{}, Invalid("type", "未知审计事件类型 %q", eventType)
	}
	raw, err := json.Marshal(detail)
	if err != nil {
		return AuditEvent{}, Storage("审计事件 %q 明细序列化失败：%v", eventType, err)
	}
	return AuditEvent{
		Seq:        seq,
		ID:         AuditEventID(seq),
		Type:       eventType,
		ApiaryID:   TrimText(apiaryID),
		HiveID:     TrimText(hiveID),
		RequestID:  requestID,
		OccurredAt: occurredAt.UTC(),
		Detail:     raw,
	}, nil
}

func validAuditEventType(eventType AuditEventType) bool {
	switch eventType {
	case AuditApiaryRegistered,
		AuditHiveInstalled,
		AuditHiveStateAdvanced,
		AuditInspectionRecorded,
		AuditQueenReplaced:
		return true
	default:
		return false
	}
}

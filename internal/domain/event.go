package domain

import "time"

// EventType 标记产生审计事实的写入类别。
type EventType string

const (
	EventApiaryRegistered   EventType = "apiary_registered"
	EventHiveInstalled      EventType = "hive_installed"
	EventHiveStateAdvanced  EventType = "hive_state_advanced"
	EventInspectionRecorded EventType = "inspection_recorded"
	EventQueenReplaced      EventType = "queen_replaced"
)

// Event 是一次被采纳的写入所留下的审计事实。事件只追加、不改写，
// 顺序号在仓储内单调递增，同一范围的重复读取得到稳定的顺序与内容。
type Event struct {
	Seq        int64          `json:"seq"`
	Type       EventType      `json:"type"`
	ApiaryID   string         `json:"apiaryId"`
	HiveID     string         `json:"hiveId,omitempty"`
	OccurredAt time.Time      `json:"occurredAt"`
	Facts      map[string]any `json:"facts"`
}

// ApiaryRegisteredEvent 记录蜂场录入事实，事实取自已写入的蜂场。
func ApiaryRegisteredEvent(seq int64, occurredAt time.Time, apiary Apiary) Event {
	return Event{
		Seq:        seq,
		Type:       EventApiaryRegistered,
		ApiaryID:   apiary.ID,
		OccurredAt: occurredAt.UTC(),
		Facts: map[string]any{
			"name":     apiary.Name,
			"district": apiary.District,
			"keeper":   apiary.Keeper,
		},
	}
}

// HiveInstalledEvent 记录蜂箱安装事实，事实取自已写入的蜂箱。
func HiveInstalledEvent(seq int64, occurredAt time.Time, hive Hive) Event {
	return Event{
		Seq:        seq,
		Type:       EventHiveInstalled,
		ApiaryID:   hive.ApiaryID,
		HiveID:     hive.ID,
		OccurredAt: occurredAt.UTC(),
		Facts: map[string]any{
			"model": string(hive.Model),
			"state": string(hive.State),
		},
	}
}

// HiveStateAdvancedEvent 记录一次被采纳的蜂群状态迁移。
func HiveStateAdvancedEvent(seq int64, occurredAt time.Time, apiaryID string, transition Transition) Event {
	return Event{
		Seq:        seq,
		Type:       EventHiveStateAdvanced,
		ApiaryID:   apiaryID,
		HiveID:     transition.HiveID,
		OccurredAt: occurredAt.UTC(),
		Facts: map[string]any{
			"from": string(transition.From),
			"to":   string(transition.To),
		},
	}
}

// InspectionRecordedEvent 记录一次被保存的巡检，事实与巡检记录同源。
func InspectionRecordedEvent(seq int64, occurredAt time.Time, apiaryID string, inspection Inspection) Event {
	return Event{
		Seq:        seq,
		Type:       EventInspectionRecorded,
		ApiaryID:   apiaryID,
		HiveID:     inspection.HiveID,
		OccurredAt: occurredAt.UTC(),
		Facts: map[string]any{
			"inspectionId":   inspection.ID,
			"observedAt":     inspection.ObservedAt.UTC(),
			"colonyStrength": inspection.ColonyStrength,
			"broodPattern":   string(inspection.BroodPattern),
			"temper":         string(inspection.Temper),
			"miteCount":      inspection.MiteCount,
			"note":           inspection.Note,
			"concerns":       append([]string{}, inspection.Concerns...),
		},
	}
}

// QueenReplacedEvent 记录一次换王：新蜂王记录与被关闭的旧蜂王编号。
// 没有旧蜂王时不携带 closedQueenId 与 closedState。
func QueenReplacedEvent(seq int64, occurredAt time.Time, apiaryID string, queen Queen, closedQueenID string, closedState QueenState) Event {
	facts := map[string]any{
		"queenId":      queen.ID,
		"introducedAt": queen.IntroducedAt.UTC(),
		"origin":       string(queen.Origin),
		"markColor":    queen.MarkColor,
		"state":        string(queen.State),
	}
	if closedQueenID != "" {
		facts["closedQueenId"] = closedQueenID
		facts["closedState"] = string(closedState)
	}
	return Event{
		Seq:        seq,
		Type:       EventQueenReplaced,
		ApiaryID:   apiaryID,
		HiveID:     queen.HiveID,
		OccurredAt: occurredAt.UTC(),
		Facts:      facts,
	}
}

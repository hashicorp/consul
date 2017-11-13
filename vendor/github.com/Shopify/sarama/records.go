package sarama

import "fmt"

const (
	legacyRecords = iota
	defaultRecords
)

// Records implements a union type containing either a RecordBatch or a legacy MessageSet.
type Records struct {
	recordsType int
	msgSet      *MessageSet
	recordBatch *RecordBatch
}

func newLegacyRecords(msgSet *MessageSet) Records {
	return Records{recordsType: legacyRecords, msgSet: msgSet}
}

func newDefaultRecords(batch *RecordBatch) Records {
	return Records{recordsType: defaultRecords, recordBatch: batch}
}

func (r *Records) encode(pe packetEncoder) error {
	switch r.recordsType {
	case legacyRecords:
		if r.msgSet == nil {
			return nil
		}
		return r.msgSet.encode(pe)
	case defaultRecords:
		if r.recordBatch == nil {
			return nil
		}
		return r.recordBatch.encode(pe)
	}
	return fmt.Errorf("unknown records type: %v", r.recordsType)
}

func (r *Records) decode(pd packetDecoder) error {
	switch r.recordsType {
	case legacyRecords:
		r.msgSet = &MessageSet{}
		return r.msgSet.decode(pd)
	case defaultRecords:
		r.recordBatch = &RecordBatch{}
		return r.recordBatch.decode(pd)
	}
	return fmt.Errorf("unknown records type: %v", r.recordsType)
}

func (r *Records) numRecords() (int, error) {
	switch r.recordsType {
	case legacyRecords:
		if r.msgSet == nil {
			return 0, nil
		}
		return len(r.msgSet.Messages), nil
	case defaultRecords:
		if r.recordBatch == nil {
			return 0, nil
		}
		return len(r.recordBatch.Records), nil
	}
	return 0, fmt.Errorf("unknown records type: %v", r.recordsType)
}

func (r *Records) isPartial() (bool, error) {
	switch r.recordsType {
	case legacyRecords:
		if r.msgSet == nil {
			return false, nil
		}
		return r.msgSet.PartialTrailingMessage, nil
	case defaultRecords:
		if r.recordBatch == nil {
			return false, nil
		}
		return r.recordBatch.PartialTrailingRecord, nil
	}
	return false, fmt.Errorf("unknown records type: %v", r.recordsType)
}

func (r *Records) isControl() (bool, error) {
	switch r.recordsType {
	case legacyRecords:
		return false, nil
	case defaultRecords:
		if r.recordBatch == nil {
			return false, nil
		}
		return r.recordBatch.Control, nil
	}
	return false, fmt.Errorf("unknown records type: %v", r.recordsType)
}

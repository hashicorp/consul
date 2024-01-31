// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package pbresource

func (o WatchEvent_Operation) IsFramingEvent() bool {
	return o == WatchEvent_OPERATION_START_OF_SNAPSHOT ||
		o == WatchEvent_OPERATION_END_OF_SNAPSHOT
}

func (o WatchEvent_Operation) GoString() string {
	return o.String()
}

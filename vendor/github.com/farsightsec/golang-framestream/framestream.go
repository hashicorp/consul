/*
 * Copyright (c) 2014 by Farsight Security, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package framestream

import (
	"errors"
	"io"
)

const DEFAULT_MAX_PAYLOAD_SIZE = 1048576
const MAX_CONTROL_FRAME_SIZE = 512

var EOF = io.EOF
var ErrContentTypeMismatch = errors.New("content type mismatch")
var ErrDataFrameTooLarge = errors.New("data frame too large")
var ErrShortRead = errors.New("short read")
var ErrDecode = errors.New("decoding error")
var ErrType = errors.New("invalid type")

// Licensed to the Apache Software Foundation(ASF) under one
// or more contributor license agreements.See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership.The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
// with the License. You may obtain a copy of the License at
// 
//     http://www.apache.org/licenses/LICENSE-2.0
// 
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

#include <thrift/parse/t_program.h>

class TestDataGenerator
{
public:
    static const string DEFAULT_FILE_HEADER;

    static std::pair<string, t_enum*> get_test_enum_data(t_program* program);
    static std::pair<string, t_const*> get_test_void_const_data(t_netcore_generator* gen);
    static std::pair<string, t_const*> get_test_string_const_data(t_netcore_generator* gen);
    static std::pair<string, t_const*> get_test_bool_const_data(t_netcore_generator* gen);
    static std::pair<string, t_const*> get_test_i8_const_data(t_netcore_generator* gen);
    static std::pair<string, t_const*> get_test_i16_const_data(t_netcore_generator* gen);
    static std::pair<string, t_const*> get_test_i32_const_data(t_netcore_generator* gen);
    static std::pair<string, t_const*> get_test_i64_const_data(t_netcore_generator* gen);
    static std::pair<string, t_const*> get_test_double_const_data(t_netcore_generator* gen);
};

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

#include "../catch/catch.hpp"
#include <thrift/parse/t_program.h>
#include <thrift/generate/t_netcore_generator.h>

TEST_CASE( "t_netcore_generator should throw error with unknown options", "[initialization]" )
{
    string path = "CassandraTest.thrift";
    string name = "netcore";
    map<string, string> parsed_options = { { "keys", "keys" } };
    string option_string = "";

    t_program* program = new t_program(path, name);
    t_netcore_generator* gen = nullptr;

    REQUIRE_THROWS(gen = new t_netcore_generator(program, parsed_options, option_string));	

    delete gen;
    delete program;	
}

TEST_CASE("t_netcore_generator should create valid instance with valid options", "[initialization]")
{
    string path = "CassandraTest.thrift";
    string name = "netcore";
    map<string, string> parsed_options = { { "wcf", "wcf" }, { "nullable", "nullable"} };
    string option_string = "";

    t_program* program = new t_program(path, name);
    t_netcore_generator* gen = nullptr;

    REQUIRE_NOTHROW(gen = new t_netcore_generator(program, parsed_options, option_string));
    REQUIRE(gen != nullptr);
    REQUIRE(gen->is_wcf_enabled());
    REQUIRE(gen->is_nullable_enabled());
    REQUIRE_FALSE(gen->is_hashcode_enabled());
    REQUIRE_FALSE(gen->is_serialize_enabled());
    REQUIRE_FALSE(gen->is_union_enabled());

    delete gen;
    delete program;
}

TEST_CASE("t_netcore_generator should pass init succesfully", "[initialization]")
{
    string path = "CassandraTest.thrift";
    string name = "netcore";
    map<string, string> parsed_options = { { "wcf", "wcf" },{ "nullable", "nullable" } };
    string option_string = "";

    t_program* program = new t_program(path, name);
    t_netcore_generator* gen = new t_netcore_generator(program, parsed_options, option_string);

    REQUIRE_NOTHROW(gen->init_generator());

    delete gen;
    delete program;
}

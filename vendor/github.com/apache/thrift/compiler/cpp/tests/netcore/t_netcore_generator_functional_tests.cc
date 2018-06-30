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
#include "t_netcore_generator_functional_tests_helpers.h"

TEST_CASE( "t_netcore_generator should generate valid enum", "[functional]" )
{
    string path = "CassandraTest.thrift";
    string name = "netcore";
    map<string, string> parsed_options = { { "wcf", "wcf" } };
    string option_string = "";

    t_program* program = new t_program(path, name);
    t_netcore_generator* gen = new t_netcore_generator(program, parsed_options, option_string);

    std::pair<string, t_enum*> pair = TestDataGenerator::get_test_enum_data(program);
    string expected_result = pair.first;
    t_enum* test_enum = pair.second;

    string file_path = test_enum->get_name() + ".cs";
    ofstream out;
    out.open(file_path.c_str());

    REQUIRE_NOTHROW(gen->generate_enum(out, test_enum));

    out.close();

    std::ifstream ifs(file_path);
    string actual_result((std::istreambuf_iterator<char>(ifs)), (std::istreambuf_iterator<char>()));
    std::remove(file_path.c_str());

    REQUIRE(expected_result == actual_result);

    delete test_enum;
    delete gen;
    delete program;	
}

TEST_CASE("t_netcore_generator should generate valid void", "[functional]")
{
    string path = "CassandraTest.thrift";
    string name = "netcore";
    map<string, string> parsed_options = { { "wcf", "wcf" } };
    string option_string = "";

    t_program* program = new t_program(path, name);
    t_netcore_generator* gen = new t_netcore_generator(program, parsed_options, option_string);

    std::pair<string, t_const*> pair = TestDataGenerator::get_test_void_const_data(gen);
    string expected_result = pair.first;
    t_const* const_ = pair.second;
    vector<t_const*> consts_;
    consts_.push_back(const_);

    string file_path = const_->get_name() + ".cs";
    ofstream out;
    out.open(file_path.c_str());

    REQUIRE_THROWS(gen->generate_consts(out, consts_));

    out.close();

    std::ifstream ifs(file_path);
    string actual_result((std::istreambuf_iterator<char>(ifs)), (std::istreambuf_iterator<char>()));
    std::remove(file_path.c_str());

    delete const_;
    delete gen;
    delete program;
}

TEST_CASE("t_netcore_generator should generate valid string with escaping keyword", "[functional]")
{
    string path = "CassandraTest.thrift";
    string name = "netcore";
    map<string, string> parsed_options = { { "wcf", "wcf" } };
    string option_string = "";

    t_program* program = new t_program(path, name);
    t_netcore_generator* gen = new t_netcore_generator(program, parsed_options, option_string);
    gen->init_generator();

    std::pair<string, t_const*> pair = TestDataGenerator::get_test_string_const_data(gen);
    string expected_result = pair.first;
    t_const* const_ = pair.second;
    vector<t_const*> consts_;
    consts_.push_back(const_);

    string file_path = const_->get_name() + ".cs";
    ofstream out;
    out.open(file_path.c_str());

    REQUIRE_NOTHROW(gen->generate_consts(out, consts_));

    out.close();

    std::ifstream ifs(file_path);
    string actual_result((std::istreambuf_iterator<char>(ifs)), (std::istreambuf_iterator<char>()));
    std::remove(file_path.c_str());

    REQUIRE(expected_result == actual_result);

    delete const_;
    delete gen;
    delete program;
}

TEST_CASE("t_netcore_generator should generate valid bool with escaping keyword", "[functional]")
{
    string path = "CassandraTest.thrift";
    string name = "netcore";
    map<string, string> parsed_options = { { "wcf", "wcf" } };
    string option_string = "";

    t_program* program = new t_program(path, name);
    t_netcore_generator* gen = new t_netcore_generator(program, parsed_options, option_string);
    gen->init_generator();

    std::pair<string, t_const*> pair = TestDataGenerator::get_test_bool_const_data(gen);
    string expected_result = pair.first;
    t_const* const_ = pair.second;
    vector<t_const*> consts_;
    consts_.push_back(const_);

    string file_path = const_->get_name() + ".cs";
    ofstream out;
    out.open(file_path.c_str());

    REQUIRE_NOTHROW(gen->generate_consts(out, consts_));

    out.close();

    std::ifstream ifs(file_path);
    string actual_result((std::istreambuf_iterator<char>(ifs)), (std::istreambuf_iterator<char>()));
    std::remove(file_path.c_str());

    REQUIRE(expected_result == actual_result);

    delete const_;
    delete gen;
    delete program;
}

TEST_CASE("t_netcore_generator should generate valid sbyte (i8) with escaping keyword", "[functional]")
{
    string path = "CassandraTest.thrift";
    string name = "netcore";
    map<string, string> parsed_options = { { "wcf", "wcf" } };
    string option_string = "";

    t_program* program = new t_program(path, name);
    t_netcore_generator* gen = new t_netcore_generator(program, parsed_options, option_string);
    gen->init_generator();

    std::pair<string, t_const*> pair = TestDataGenerator::get_test_i8_const_data(gen);
    string expected_result = pair.first;
    t_const* const_ = pair.second;
    vector<t_const*> consts_;
    consts_.push_back(const_);

    string file_path = const_->get_name() + ".cs";
    ofstream out;
    out.open(file_path.c_str());

    REQUIRE_NOTHROW(gen->generate_consts(out, consts_));

    out.close();

    std::ifstream ifs(file_path);
    string actual_result((std::istreambuf_iterator<char>(ifs)), (std::istreambuf_iterator<char>()));
    std::remove(file_path.c_str());

    REQUIRE(expected_result == actual_result);

    delete const_;
    delete gen;
    delete program;
}

TEST_CASE("t_netcore_generator should generate valid short (i16) with escaping keyword", "[functional]")
{
    string path = "CassandraTest.thrift";
    string name = "netcore";
    map<string, string> parsed_options = { { "wcf", "wcf" } };
    string option_string = "";

    t_program* program = new t_program(path, name);
    t_netcore_generator* gen = new t_netcore_generator(program, parsed_options, option_string);
    gen->init_generator();

    std::pair<string, t_const*> pair = TestDataGenerator::get_test_i16_const_data(gen);
    string expected_result = pair.first;
    t_const* const_ = pair.second;
    vector<t_const*> consts_;
    consts_.push_back(const_);

    string file_path = const_->get_name() + ".cs";
    ofstream out;
    out.open(file_path.c_str());

    REQUIRE_NOTHROW(gen->generate_consts(out, consts_));

    out.close();

    std::ifstream ifs(file_path);
    string actual_result((std::istreambuf_iterator<char>(ifs)), (std::istreambuf_iterator<char>()));
    std::remove(file_path.c_str());

    REQUIRE(expected_result == actual_result);

    delete const_;
    delete gen;
    delete program;
}

TEST_CASE("t_netcore_generator should generate valid integer (i32) with escaping keyword", "[functional]")
{
    string path = "CassandraTest.thrift";
    string name = "netcore";
    map<string, string> parsed_options = { { "wcf", "wcf" } };
    string option_string = "";

    t_program* program = new t_program(path, name);
    t_netcore_generator* gen = new t_netcore_generator(program, parsed_options, option_string);
    gen->init_generator();

    std::pair<string, t_const*> pair = TestDataGenerator::get_test_i32_const_data(gen);
    string expected_result = pair.first;
    t_const* const_ = pair.second;
    vector<t_const*> consts_;
    consts_.push_back(const_);

    string file_path = const_->get_name() + ".cs";
    ofstream out;
    out.open(file_path.c_str());

    REQUIRE_NOTHROW(gen->generate_consts(out, consts_));

    out.close();

    std::ifstream ifs(file_path);
    string actual_result((std::istreambuf_iterator<char>(ifs)), (std::istreambuf_iterator<char>()));
    std::remove(file_path.c_str());

    REQUIRE(expected_result == actual_result);

    delete const_;
    delete gen;
    delete program;
}

TEST_CASE("t_netcore_generator should generate valid long (i64) with escaping keyword", "[functional]")
{
    string path = "CassandraTest.thrift";
    string name = "netcore";
    map<string, string> parsed_options = { { "wcf", "wcf" } };
    string option_string = "";

    t_program* program = new t_program(path, name);
    t_netcore_generator* gen = new t_netcore_generator(program, parsed_options, option_string);
    gen->init_generator();

    std::pair<string, t_const*> pair = TestDataGenerator::get_test_i64_const_data(gen);
    string expected_result = pair.first;
    t_const* const_ = pair.second;
    vector<t_const*> consts_;
    consts_.push_back(const_);

    string file_path = const_->get_name() + ".cs";
    ofstream out;
    out.open(file_path.c_str());

    REQUIRE_NOTHROW(gen->generate_consts(out, consts_));

    out.close();

    std::ifstream ifs(file_path);
    string actual_result((std::istreambuf_iterator<char>(ifs)), (std::istreambuf_iterator<char>()));
    std::remove(file_path.c_str());

    REQUIRE(expected_result == actual_result);

    delete const_;
    delete gen;
    delete program;
}

TEST_CASE("t_netcore_generator should generate valid double with escaping keyword", "[functional]")
{
    string path = "CassandraTest.thrift";
    string name = "netcore";
    map<string, string> parsed_options = { { "wcf", "wcf" } };
    string option_string = "";

    t_program* program = new t_program(path, name);
    t_netcore_generator* gen = new t_netcore_generator(program, parsed_options, option_string);
    gen->init_generator();

    std::pair<string, t_const*> pair = TestDataGenerator::get_test_double_const_data(gen);
    string expected_result = pair.first;
    t_const* const_ = pair.second;
    vector<t_const*> consts_;
    consts_.push_back(const_);

    string file_path = const_->get_name() + ".cs";
    ofstream out;
    out.open(file_path.c_str());

    REQUIRE_NOTHROW(gen->generate_consts(out, consts_));

    out.close();

    std::ifstream ifs(file_path);
    string actual_result((std::istreambuf_iterator<char>(ifs)), (std::istreambuf_iterator<char>()));
    std::remove(file_path.c_str());

    REQUIRE(expected_result == actual_result);

    delete const_;
    delete gen;
    delete program;
}

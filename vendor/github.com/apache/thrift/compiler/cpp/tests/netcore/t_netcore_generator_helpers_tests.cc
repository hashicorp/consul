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

using std::vector;

TEST_CASE("t_netcore_generator::netcore_type_usings() without option wcf should return valid namespaces", "[helpers]")
{
    string path = "CassandraTest.thrift";
    string name = "netcore";
    map<string, string> parsed_options = { { "union", "union" } };
    string option_string = "";

    string expected_namespaces = "using System;\n"
                                "using System.Collections;\n"
                                "using System.Collections.Generic;\n"
                                "using System.Text;\n"
                                "using System.IO;\n"
                                "using System.Threading;\n"
                                "using System.Threading.Tasks;\n"
                                "using Thrift;\n"
                                "using Thrift.Collections;\n" + endl;

    t_program* program = new t_program(path, name);
    t_netcore_generator* gen = new t_netcore_generator(program, parsed_options, option_string);

    REQUIRE_FALSE(gen->is_wcf_enabled());
    REQUIRE(gen->netcore_type_usings() == expected_namespaces);

    delete gen;
    delete program;
}

TEST_CASE("t_netcore_generator::netcore_type_usings() with option wcf should return valid namespaces", "[helpers]")
{
    string path = "CassandraTest.thrift";
    string name = "netcore";
    map<string, string> parsed_options = { { "wcf", "wcf" } };
    string option_string = "";

    string expected_namespaces_wcf = "using System;\n"
                                    "using System.Collections;\n"
                                    "using System.Collections.Generic;\n"
                                    "using System.Text;\n"
                                    "using System.IO;\n"
                                    "using System.Threading;\n"
                                    "using System.Threading.Tasks;\n"
                                    "using Thrift;\n"
                                    "using Thrift.Collections;\n"
                                    "using System.ServiceModel;\n"
                                    "using System.Runtime.Serialization;\n" + endl;

    t_program* program = new t_program(path, name);
    t_netcore_generator* gen = new t_netcore_generator(program, parsed_options, option_string);

    REQUIRE(gen->is_wcf_enabled());
    REQUIRE(gen->netcore_type_usings() == expected_namespaces_wcf);

    delete gen;
    delete program;
}

TEST_CASE("t_netcore_generator should contains latest C# keywords to normalize with @", "[helpers]")
{
    string path = "CassandraTest.thrift";
    string name = "netcore";
    map<string, string> parsed_options = { { "wcf", "wcf" } };
    string option_string = "";
    vector<string> current_keywords = {
        { "abstract" },
        { "as" },
        { "base" },
        { "bool" },
        { "break" },
        { "byte" },
        { "case" },
        { "catch" },
        { "char" },
        { "checked" },
        { "class" },
        { "const" },
        { "continue" },
        { "decimal" },
        { "default" },
        { "delegate" },
        { "do" },
        { "double" },
        { "else" },
        { "enum" },
        { "event" },
        { "explicit" },
        { "extern" },
        { "false" },
        { "finally" },
        { "fixed" },
        { "float" },
        { "for" },
        { "foreach" },
        { "goto" },
        { "if" },
        { "implicit" },
        { "in" },
        { "int" },
        { "interface" },
        { "internal" },
        { "is" },
        { "lock" },
        { "long" },
        { "namespace" },
        { "new" },
        { "null" },
        { "object" },
        { "operator" },
        { "out" },
        { "override" },
        { "params" },
        { "private" },
        { "protected" },
        { "public" },
        { "readonly" },
        { "ref" },
        { "return" },
        { "sbyte" },
        { "sealed" },
        { "short" },
        { "sizeof" },
        { "stackalloc" },
        { "static" },
        { "string" },
        { "struct" },
        { "switch" },
        { "this" },
        { "throw" },
        { "true" },
        { "try" },
        { "typeof" },
        { "uint" },
        { "ulong" },
        { "unchecked" },
        { "unsafe" },
        { "ushort" },
        { "using" },
        { "void" },
        { "volatile" },
        { "while" },
        // Contextual Keywords
        { "add" },
        { "alias" },
        { "ascending" },
        { "async" },
        { "await" },
        { "descending" },
        { "dynamic" },
        { "from" },
        { "get" },
        { "global" },
        { "group" },
        { "into" },
        { "join" },
        { "let" },
        { "orderby" },
        { "partial" },
        { "remove" },
        { "select" },
        { "set" },
        { "value" },
        { "var" },
        { "when" },
        { "where" },
        { "yield" }
    };

    string missed_keywords = "";

    t_program* program = new t_program(path, name);
    t_netcore_generator* gen = new t_netcore_generator(program, parsed_options, option_string);
    gen->init_generator();
    map<string, int> generators_keywords = gen->get_keywords_list();

    for (vector<string>::iterator it = current_keywords.begin(); it != current_keywords.end(); ++it)
    {
        if (generators_keywords.find(*it) == generators_keywords.end())
        {
            missed_keywords = missed_keywords + *it + ",";
        }
    }

    REQUIRE(missed_keywords == "");

    delete gen;
    delete program;
}

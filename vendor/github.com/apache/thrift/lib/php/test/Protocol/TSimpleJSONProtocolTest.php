<?php

/*
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements. See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership. The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License. You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 *
 * @package thrift.test
 */

namespace Test\Thrift\Protocol;

use PHPUnit\Framework\TestCase;
use Test\Thrift\Fixtures;
use Thrift\Protocol\TSimpleJSONProtocol;
use Thrift\Transport\TMemoryBuffer;

require __DIR__ . '/../../../../vendor/autoload.php';

/***
 * This test suite depends on running the compiler against the
 * standard ThriftTest.thrift file:
 *
 * lib/php/test$ ../../../compiler/cpp/thrift --gen php -r \
 *   --out ./packages ../../../test/ThriftTest.thrift
 *
 * @runTestsInSeparateProcesses
 */
class TSimpleJSONProtocolTest extends TestCase
{
    private $transport;
    private $protocol;

    public static function setUpBeforeClass()
    {

        /** @var \Composer\Autoload\ClassLoader $loader */
        $loader = require __DIR__ . '/../../../../vendor/autoload.php';
        $loader->addPsr4('', __DIR__ . '/../packages/php');

        Fixtures::populateTestArgs();
        TSimpleJSONProtocolFixtures::populateTestArgsSimpleJSON();
    }

    public function setUp()
    {
        $this->transport = new TMemoryBuffer();
        $this->protocol = new TSimpleJSONProtocol($this->transport);
        $this->transport->open();
    }

    /**
     * WRITE TESTS
     */
    public function testVoidWrite()
    {
        $args = new \ThriftTest\ThriftTest_testVoid_args();
        $args->write($this->protocol);

        $actual = $this->transport->read(Fixtures::$bufsize);
        $expected = TSimpleJSONProtocolFixtures::$testArgsJSON['testVoid'];

        $this->assertEquals($expected, $actual);
    }

    public function testString1Write()
    {
        $args = new \ThriftTest\ThriftTest_testString_args();
        $args->thing = Fixtures::$testArgs['testString1'];
        $args->write($this->protocol);

        $actual = $this->transport->read(Fixtures::$bufsize);
        $expected = TSimpleJSONProtocolFixtures::$testArgsJSON['testString1'];

        $this->assertEquals($expected, $actual);
    }

    public function testString2Write()
    {
        $args = new \ThriftTest\ThriftTest_testString_args();
        $args->thing = Fixtures::$testArgs['testString2'];
        $args->write($this->protocol);

        $actual = $this->transport->read(Fixtures::$bufsize);
        $expected = TSimpleJSONProtocolFixtures::$testArgsJSON['testString2'];

        $this->assertEquals($expected, $actual);
    }

    public function testDoubleWrite()
    {
        $args = new \ThriftTest\ThriftTest_testDouble_args();
        $args->thing = Fixtures::$testArgs['testDouble'];
        $args->write($this->protocol);

        $actual = $this->transport->read(Fixtures::$bufsize);
        $expected = TSimpleJSONProtocolFixtures::$testArgsJSON['testDouble'];

        $this->assertEquals($expected, $actual);
    }

    public function testByteWrite()
    {
        $args = new \ThriftTest\ThriftTest_testByte_args();
        $args->thing = Fixtures::$testArgs['testByte'];
        $args->write($this->protocol);

        $actual = $this->transport->read(Fixtures::$bufsize);
        $expected = TSimpleJSONProtocolFixtures::$testArgsJSON['testByte'];

        $this->assertEquals($expected, $actual);
    }

    public function testI32Write()
    {
        $args = new \ThriftTest\ThriftTest_testI32_args();
        $args->thing = Fixtures::$testArgs['testI32'];
        $args->write($this->protocol);

        $actual = $this->transport->read(Fixtures::$bufsize);
        $expected = TSimpleJSONProtocolFixtures::$testArgsJSON['testI32'];

        $this->assertEquals($expected, $actual);
    }

    public function testI64Write()
    {
        $args = new \ThriftTest\ThriftTest_testI64_args();
        $args->thing = Fixtures::$testArgs['testI64'];
        $args->write($this->protocol);

        $actual = $this->transport->read(Fixtures::$bufsize);
        $expected = TSimpleJSONProtocolFixtures::$testArgsJSON['testI64'];

        $this->assertEquals($expected, $actual);
    }

    public function testStructWrite()
    {
        $args = new \ThriftTest\ThriftTest_testStruct_args();
        $args->thing = Fixtures::$testArgs['testStruct'];

        $args->write($this->protocol);

        $actual = $this->transport->read(Fixtures::$bufsize);
        $expected = TSimpleJSONProtocolFixtures::$testArgsJSON['testStruct'];

        $this->assertEquals($expected, $actual);
    }

    public function testNestWrite()
    {
        $args = new \ThriftTest\ThriftTest_testNest_args();
        $args->thing = Fixtures::$testArgs['testNest'];

        $args->write($this->protocol);

        $actual = $this->transport->read(Fixtures::$bufsize);
        $expected = TSimpleJSONProtocolFixtures::$testArgsJSON['testNest'];

        $this->assertEquals($expected, $actual);
    }

    public function testMapWrite()
    {
        $args = new \ThriftTest\ThriftTest_testMap_args();
        $args->thing = Fixtures::$testArgs['testMap'];

        $args->write($this->protocol);

        $actual = $this->transport->read(Fixtures::$bufsize);
        $expected = TSimpleJSONProtocolFixtures::$testArgsJSON['testMap'];

        $this->assertEquals($expected, $actual);
    }

    public function testStringMapWrite()
    {
        $args = new \ThriftTest\ThriftTest_testStringMap_args();
        $args->thing = Fixtures::$testArgs['testStringMap'];

        $args->write($this->protocol);

        $actual = $this->transport->read(Fixtures::$bufsize);
        $expected = TSimpleJSONProtocolFixtures::$testArgsJSON['testStringMap'];

        $this->assertEquals($expected, $actual);
    }

    public function testSetWrite()
    {
        $args = new \ThriftTest\ThriftTest_testSet_args();
        $args->thing = Fixtures::$testArgs['testSet'];

        $args->write($this->protocol);

        $actual = $this->transport->read(Fixtures::$bufsize);
        $expected = TSimpleJSONProtocolFixtures::$testArgsJSON['testSet'];

        $this->assertEquals($expected, $actual);
    }

    public function testListWrite()
    {
        $args = new \ThriftTest\ThriftTest_testList_args();
        $args->thing = Fixtures::$testArgs['testList'];

        $args->write($this->protocol);

        $actual = $this->transport->read(Fixtures::$bufsize);
        $expected = TSimpleJSONProtocolFixtures::$testArgsJSON['testList'];

        $this->assertEquals($expected, $actual);
    }

    public function testEnumWrite()
    {
        $args = new \ThriftTest\ThriftTest_testEnum_args();
        $args->thing = Fixtures::$testArgs['testEnum'];

        $args->write($this->protocol);

        $actual = $this->transport->read(Fixtures::$bufsize);
        $expected = TSimpleJSONProtocolFixtures::$testArgsJSON['testEnum'];

        $this->assertEquals($expected, $actual);
    }

    public function testTypedefWrite()
    {
        $args = new \ThriftTest\ThriftTest_testTypedef_args();
        $args->thing = Fixtures::$testArgs['testTypedef'];

        $args->write($this->protocol);

        $actual = $this->transport->read(Fixtures::$bufsize);
        $expected = TSimpleJSONProtocolFixtures::$testArgsJSON['testTypedef'];

        $this->assertEquals($expected, $actual);
    }
}

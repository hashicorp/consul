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
use Thrift\Protocol\TJSONProtocol;
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
class TJSONProtocolTest extends TestCase
{
    private $transport;
    private $protocol;

    public static function setUpBeforeClass()
    {
        /** @var \Composer\Autoload\ClassLoader $loader */
        $loader = require __DIR__ . '/../../../../vendor/autoload.php';
        $loader->addPsr4('', __DIR__ . '/../packages/php');

        Fixtures::populateTestArgs();
        TJSONProtocolFixtures::populateTestArgsJSON();
    }

    public function setUp()
    {
        $this->transport = new TMemoryBuffer();
        $this->protocol = new TJSONProtocol($this->transport);
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
        $expected = TJSONProtocolFixtures::$testArgsJSON['testVoid'];

        $this->assertEquals($expected, $actual);
    }

    public function testString1Write()
    {
        $args = new \ThriftTest\ThriftTest_testString_args();
        $args->thing = Fixtures::$testArgs['testString1'];
        $args->write($this->protocol);

        $actual = $this->transport->read(Fixtures::$bufsize);
        $expected = TJSONProtocolFixtures::$testArgsJSON['testString1'];

        $this->assertEquals($expected, $actual);
    }

    public function testString2Write()
    {
        $args = new \ThriftTest\ThriftTest_testString_args();
        $args->thing = Fixtures::$testArgs['testString2'];
        $args->write($this->protocol);

        $actual = $this->transport->read(Fixtures::$bufsize);
        $expected = TJSONProtocolFixtures::$testArgsJSON['testString2'];

        $this->assertEquals($expected, $actual);
    }

    public function testDoubleWrite()
    {
        $args = new \ThriftTest\ThriftTest_testDouble_args();
        $args->thing = Fixtures::$testArgs['testDouble'];
        $args->write($this->protocol);

        $actual = $this->transport->read(Fixtures::$bufsize);
        $expected = TJSONProtocolFixtures::$testArgsJSON['testDouble'];

        $this->assertEquals($expected, $actual);
    }

    public function testByteWrite()
    {
        $args = new \ThriftTest\ThriftTest_testByte_args();
        $args->thing = Fixtures::$testArgs['testByte'];
        $args->write($this->protocol);

        $actual = $this->transport->read(Fixtures::$bufsize);
        $expected = TJSONProtocolFixtures::$testArgsJSON['testByte'];

        $this->assertEquals($expected, $actual);
    }

    public function testI32Write()
    {
        $args = new \ThriftTest\ThriftTest_testI32_args();
        $args->thing = Fixtures::$testArgs['testI32'];
        $args->write($this->protocol);

        $actual = $this->transport->read(Fixtures::$bufsize);
        $expected = TJSONProtocolFixtures::$testArgsJSON['testI32'];

        $this->assertEquals($expected, $actual);
    }

    public function testI64Write()
    {
        $args = new \ThriftTest\ThriftTest_testI64_args();
        $args->thing = Fixtures::$testArgs['testI64'];
        $args->write($this->protocol);

        $actual = $this->transport->read(Fixtures::$bufsize);
        $expected = TJSONProtocolFixtures::$testArgsJSON['testI64'];

        $this->assertEquals($expected, $actual);
    }

    public function testStructWrite()
    {
        $args = new \ThriftTest\ThriftTest_testStruct_args();
        $args->thing = Fixtures::$testArgs['testStruct'];

        $args->write($this->protocol);

        $actual = $this->transport->read(Fixtures::$bufsize);
        $expected = TJSONProtocolFixtures::$testArgsJSON['testStruct'];

        $this->assertEquals($expected, $actual);
    }

    public function testNestWrite()
    {
        $args = new \ThriftTest\ThriftTest_testNest_args();
        $args->thing = Fixtures::$testArgs['testNest'];

        $args->write($this->protocol);

        $actual = $this->transport->read(Fixtures::$bufsize);
        $expected = TJSONProtocolFixtures::$testArgsJSON['testNest'];

        $this->assertEquals($expected, $actual);
    }

    public function testMapWrite()
    {
        $args = new \ThriftTest\ThriftTest_testMap_args();
        $args->thing = Fixtures::$testArgs['testMap'];

        $args->write($this->protocol);

        $actual = $this->transport->read(Fixtures::$bufsize);
        $expected = TJSONProtocolFixtures::$testArgsJSON['testMap'];

        $this->assertEquals($expected, $actual);
    }

    public function testStringMapWrite()
    {
        $args = new \ThriftTest\ThriftTest_testStringMap_args();
        $args->thing = Fixtures::$testArgs['testStringMap'];

        $args->write($this->protocol);

        $actual = $this->transport->read(Fixtures::$bufsize);
        $expected = TJSONProtocolFixtures::$testArgsJSON['testStringMap'];

        /*
         * The $actual returns unescaped string.
         * It is required to to decode then encode it again
         * to get the expected escaped unicode.
         */
        $this->assertEquals($expected, json_encode(json_decode($actual)));
    }

    public function testSetWrite()
    {
        $args = new \ThriftTest\ThriftTest_testSet_args();
        $args->thing = Fixtures::$testArgs['testSet'];

        $args->write($this->protocol);

        $actual = $this->transport->read(Fixtures::$bufsize);
        $expected = TJSONProtocolFixtures::$testArgsJSON['testSet'];

        $this->assertEquals($expected, $actual);
    }

    public function testListWrite()
    {
        $args = new \ThriftTest\ThriftTest_testList_args();
        $args->thing = Fixtures::$testArgs['testList'];

        $args->write($this->protocol);

        $actual = $this->transport->read(Fixtures::$bufsize);
        $expected = TJSONProtocolFixtures::$testArgsJSON['testList'];

        $this->assertEquals($expected, $actual);
    }

    public function testEnumWrite()
    {
        $args = new \ThriftTest\ThriftTest_testEnum_args();
        $args->thing = Fixtures::$testArgs['testEnum'];

        $args->write($this->protocol);

        $actual = $this->transport->read(Fixtures::$bufsize);
        $expected = TJSONProtocolFixtures::$testArgsJSON['testEnum'];

        $this->assertEquals($expected, $actual);
    }

    public function testTypedefWrite()
    {
        $args = new \ThriftTest\ThriftTest_testTypedef_args();
        $args->thing = Fixtures::$testArgs['testTypedef'];

        $args->write($this->protocol);

        $actual = $this->transport->read(Fixtures::$bufsize);
        $expected = TJSONProtocolFixtures::$testArgsJSON['testTypedef'];

        $this->assertEquals($expected, $actual);
    }

    /**
     * READ TESTS
     */
    public function testVoidRead()
    {
        $this->transport->write(
            TJSONProtocolFixtures::$testArgsJSON['testVoid']
        );
        $args = new \ThriftTest\ThriftTest_testVoid_args();
        $args->read($this->protocol);
    }

    public function testString1Read()
    {
        $this->transport->write(
            TJSONProtocolFixtures::$testArgsJSON['testString1']
        );
        $args = new \ThriftTest\ThriftTest_testString_args();
        $args->read($this->protocol);

        $actual = $args->thing;
        $expected = Fixtures::$testArgs['testString1'];

        $this->assertEquals($expected, $actual);
    }

    public function testString2Read()
    {
        $this->transport->write(
            TJSONProtocolFixtures::$testArgsJSON['testString2']
        );
        $args = new \ThriftTest\ThriftTest_testString_args();
        $args->read($this->protocol);

        $actual = $args->thing;
        $expected = Fixtures::$testArgs['testString2'];

        $this->assertEquals($expected, $actual);
    }

    public function testString3Write()
    {
        $args = new \ThriftTest\ThriftTest_testString_args();
        $args->thing = Fixtures::$testArgs['testString3'];
        $args->write($this->protocol);

        $actual = $this->transport->read(Fixtures::$bufsize);
        $expected = TJSONProtocolFixtures::$testArgsJSON['testString3'];

        $this->assertEquals($expected, $actual);
    }

    public function testString4Write()
    {
        $args = new \ThriftTest\ThriftTest_testString_args();
        $args->thing = Fixtures::$testArgs['testUnicodeStringWithNonBMP'];
        $args->write($this->protocol);

        $actual = $this->transport->read(Fixtures::$bufsize);
        $expected = TJSONProtocolFixtures::$testArgsJSON['testUnicodeStringWithNonBMP'];

        $this->assertEquals($expected, $actual);
    }

    public function testDoubleRead()
    {
        $this->transport->write(
            TJSONProtocolFixtures::$testArgsJSON['testDouble']
        );
        $args = new \ThriftTest\ThriftTest_testDouble_args();
        $args->read($this->protocol);

        $actual = $args->thing;
        $expected = Fixtures::$testArgs['testDouble'];

        $this->assertEquals($expected, $actual);
    }

    public function testByteRead()
    {
        $this->transport->write(
            TJSONProtocolFixtures::$testArgsJSON['testByte']
        );
        $args = new \ThriftTest\ThriftTest_testByte_args();
        $args->read($this->protocol);

        $actual = $args->thing;
        $expected = Fixtures::$testArgs['testByte'];

        $this->assertEquals($expected, $actual);
    }

    public function testI32Read()
    {
        $this->transport->write(
            TJSONProtocolFixtures::$testArgsJSON['testI32']
        );
        $args = new \ThriftTest\ThriftTest_testI32_args();
        $args->read($this->protocol);

        $actual = $args->thing;
        $expected = Fixtures::$testArgs['testI32'];

        $this->assertEquals($expected, $actual);
    }

    public function testI64Read()
    {
        $this->transport->write(
            TJSONProtocolFixtures::$testArgsJSON['testI64']
        );
        $args = new \ThriftTest\ThriftTest_testI64_args();
        $args->read($this->protocol);

        $actual = $args->thing;
        $expected = Fixtures::$testArgs['testI64'];

        $this->assertEquals($expected, $actual);
    }

    public function testStructRead()
    {
        $this->transport->write(
            TJSONProtocolFixtures::$testArgsJSON['testStruct']
        );
        $args = new \ThriftTest\ThriftTest_testStruct_args();
        $args->read($this->protocol);

        $actual = $args->thing;
        $expected = Fixtures::$testArgs['testStruct'];

        $this->assertEquals($expected, $actual);
    }

    public function testNestRead()
    {
        $this->transport->write(
            TJSONProtocolFixtures::$testArgsJSON['testNest']
        );
        $args = new \ThriftTest\ThriftTest_testNest_args();
        $args->read($this->protocol);

        $actual = $args->thing;
        $expected = Fixtures::$testArgs['testNest'];

        $this->assertEquals($expected, $actual);
    }

    public function testMapRead()
    {
        $this->transport->write(
            TJSONProtocolFixtures::$testArgsJSON['testMap']
        );
        $args = new \ThriftTest\ThriftTest_testMap_args();
        $args->read($this->protocol);

        $actual = $args->thing;
        $expected = Fixtures::$testArgs['testMap'];

        $this->assertEquals($expected, $actual);
    }

    public function testStringMapRead()
    {
        $this->transport->write(
            TJSONProtocolFixtures::$testArgsJSON['testStringMap']
        );
        $args = new \ThriftTest\ThriftTest_testStringMap_args();
        $args->read($this->protocol);

        $actual = $args->thing;
        $expected = Fixtures::$testArgs['testStringMap'];

        $this->assertEquals($expected, $actual);
    }

    public function testSetRead()
    {
        $this->transport->write(
            TJSONProtocolFixtures::$testArgsJSON['testSet']
        );
        $args = new \ThriftTest\ThriftTest_testSet_args();
        $args->read($this->protocol);

        $actual = $args->thing;
        $expected = Fixtures::$testArgs['testSet'];

        $this->assertEquals($expected, $actual);
    }

    public function testListRead()
    {
        $this->transport->write(
            TJSONProtocolFixtures::$testArgsJSON['testList']
        );
        $args = new \ThriftTest\ThriftTest_testList_args();
        $args->read($this->protocol);

        $actual = $args->thing;
        $expected = Fixtures::$testArgs['testList'];

        $this->assertEquals($expected, $actual);
    }

    public function testEnumRead()
    {
        $this->transport->write(
            TJSONProtocolFixtures::$testArgsJSON['testEnum']
        );
        $args = new \ThriftTest\ThriftTest_testEnum_args();
        $args->read($this->protocol);

        $actual = $args->thing;
        $expected = Fixtures::$testArgs['testEnum'];

        $this->assertEquals($expected, $actual);
    }

    public function testTypedefRead()
    {
        $this->transport->write(
            TJSONProtocolFixtures::$testArgsJSON['testTypedef']
        );
        $args = new \ThriftTest\ThriftTest_testTypedef_args();
        $args->read($this->protocol);

        $actual = $args->thing;
        $expected = Fixtures::$testArgs['testTypedef'];

        $this->assertEquals($expected, $actual);
    }

    public function testMapMapRead()
    {
        $this->transport->write(
            TJSONProtocolFixtures::$testArgsJSON['testMapMap']
        );
        $result = new \ThriftTest\ThriftTest_testMapMap_result();
        $result->read($this->protocol);

        $actual = $result->success;
        $expected = Fixtures::$testArgs['testMapMapExpectedResult'];

        $this->assertEquals($expected, $actual);
    }

    public function testInsanityRead()
    {
        $this->transport->write(
            TJSONProtocolFixtures::$testArgsJSON['testInsanity']
        );
        $result = new \ThriftTest\ThriftTest_testInsanity_result();
        $result->read($this->protocol);

        $actual = $result->success;
        $expected = Fixtures::$testArgs['testInsanityExpectedResult'];

        $this->assertEquals($expected, $actual);
    }
}

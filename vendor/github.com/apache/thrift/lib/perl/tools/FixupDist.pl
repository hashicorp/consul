#
# Licensed to the Apache Software Foundation (ASF) under one
# or more contributor license agreements. See the NOTICE file
# distributed with this work for additional information
# regarding copyright ownership. The ASF licenses this file
# to you under the Apache License, Version 2.0 (the
# "License"); you may not use this file except in compliance
# with the License. You may obtain a copy of the License at
#
#   http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied. See the License for the
# specific language governing permissions and limitations
# under the License.
#

#
# This will fix up the distribution so that CPAN properly
# indexes Thrift.
#

use 5.10.0;
use strict;
use warnings;
use utf8;

use Data::Dumper;
use CPAN::Meta;

my $meta = CPAN::Meta->load_file('META.json');
$meta->{'provides'} = { 'Thrift' => { 'file' => 'lib/Thrift.pm', 'version' => $meta->version() } };
$meta->save('META.json');

::
:: Licensed under the Apache License, Version 2.0 (the "License");
:: you may not use this file except in compliance with the License.
:: You may obtain a copy of the License at
:: 
::     http://www.apache.org/licenses/LICENSE-2.0
:: 
:: Unless required by applicable law or agreed to in writing, software
:: distributed under the License is distributed on an "AS IS" BASIS,
:: WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
:: See the License for the specific language governing permissions and
:: limitations under the License.
:: 

::
:: Set the versions of third party libraries to use.
::

IF NOT DEFINED TP_BOOST_VERSION    SET TP_BOOST_VERSION=1_62_0
IF NOT DEFINED TP_LIBEVENT_VERSION SET TP_LIBEVENT_VERSION=2.1.7rc2
IF NOT DEFINED TP_OPENSSL_VERSION  SET TP_OPENSSL_VERSION=1.1.0c
IF NOT DEFINED TP_ZLIB_VERSION     SET TP_ZLIB_VERSION=1.2.9

EXIT /B 0

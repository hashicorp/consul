---
layout: "docs"
page_title: "Windows Service"
sidebar_current: "docs-guides-windows-service"
description: |-
  By using the _sc_ command either on Powershell or 
  the Windows command line, you can make Consul run as a service. For more details about the _sc_ command
  the Windows page for [sc](https://msdn.microsoft.com/en-us/library/windows/desktop/ms682107(v=vs.85).aspx)
  should help you get started.

---

# Overview
By using the _sc_ command either on Powershell or 
the Windows command line, you can make Consul run as a service. For more details about the _sc_ command
the Windows page for [sc](https://msdn.microsoft.com/en-us/library/windows/desktop/ms682107(v=vs.85).aspx)
should help you get started.

Please remember to create a permanent directory for storing the configuration files. It is always
advisable to start Consul with the _-config-dir_ option.

The steps presented here , we assume that the user has launched Powershell with _Adminstrator_ capabilities.

## Running Consul run as a service on Windows

### Installing Consul as a Service

Download the Consul binary for your architecture.

  Use the _sc_ command to create a Service named **Consul**, which starts in the _dev_ mode.

   ```text
   sc.exe create "Consul" binPath="Path to the Consul.exe arg1 arg2 ...argN"
   [SC] CreateService SUCCESS 
   ```
   
   
   If you get an output that is similar to the one above, then your service is
   registered with the Service manager. 
   
   
   If you get an error, please check that
   you have specified the proper path to the binary and check if you've entered the arguments correctly for the Consul
   service.


### Running Consul as a service

You have two ways to start the service.

* Go to the Windows Service Manager, and look for **Consul** under the 
  service name. Click the _start_ button to start the service.
* Using the _sc_ command:
   
     ```text
     sc.exe start "Consul"  
     
     SERVICE_NAME: Consul
            TYPE               : 10  WIN32_OWN_PROCESS
            STATE              : 4  RUNNING
                                    (STOPPABLE, NOT_PAUSABLE, ACCEPTS_SHUTDOWN)
            WIN32_EXIT_CODE    : 0  (0x0)
            SERVICE_EXIT_CODE  : 0  (0x0)
            CHECKPOINT         : 0x0
            WAIT_HINT          : 0x0
            PID                : 8008
            FLAGS              : 
     ```

The service automatically starts up during/after boot, so you don't need to
launch Consul from the command-line again. 
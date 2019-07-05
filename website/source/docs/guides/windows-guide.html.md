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

# Run Consul as a Service on Windows

By using the _sc_ command, either on Powershell or 
the Windows command line, you can run Consul as a service. For more details about the _sc_ command
the Windows page for [sc](https://msdn.microsoft.com/en-us/library/windows/desktop/ms682107(v=vs.85).aspx)
should help you get started.

Before installing Consul, you will need to create a permanent directory for storing the configuration files. Once that directory is created, you will set it when starting Consul with the `-config-dir` option.

In this guide, you will download the Consul binary, register the Consul service
with the Service Manager, and finally start Consul. 

The steps presented here, assume that you have launched Powershell with _Adminstrator_ capabilities.

## Installing Consul as a Service

Download the Consul binary for your architecture.

Use the _sc_ command to create a service named **Consul**, that will load configuration files from the `config-dir`. Read the agent configuration
[documentation](/docs/agent/options.html#configuration-files) to learn more about configuration options.

```text
sc.exe create "Consul" binPath= "<path to the Consul.exe> agent -config-dir <path to configuration directory>" start= auto
[SC] CreateService SUCCESS 
```
   
If you get an output that is similar to the one above, then your service is
registered with the Service Manager. 
   
If you get an error, please check that
you have specified the proper path to the binary and check if you've entered the arguments correctly for the Consul service.


## Running Consul as a Service

You have two options for starting the service.

The first option is to use the Windows Service Manager, and look for **Consul** under the service name. Click the _start_ button to start the service.

The second option is to use the _sc_ command.
   
```text
sc.exe start "Consul"  
     
SERVICE_NAME: Consul
        TYPE               : 10  WIN32_OWN_PROCESS
        STATE              : 4  RUNNING (STOPPABLE, NOT_PAUSABLE, ACCEPTS_SHUTDOWN)
        WIN32_EXIT_CODE    : 0  (0x0)
        SERVICE_EXIT_CODE  : 0  (0x0)
        CHECKPOINT         : 0x0
        WAIT_HINT          : 0x0
        PID                : 8008
        FLAGS              : 
```

The service automatically starts up during/after boot, so you don't need to
launch Consul from the command-line again.

## Summary

In this guide you setup a Consul service on Windows. This process can be repeated to setup an entire cluster of agents. 

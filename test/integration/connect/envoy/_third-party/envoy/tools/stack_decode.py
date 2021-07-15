#!/usr/bin/env python3

# Call addr2line as needed to resolve addresses in a stack trace. The addresses
# will be replaced if they can be resolved into file and line numbers. The
# executable must include debugging information to get file and line numbers.
#
# Two ways to call:
#   1) Execute binary as a subprocess: stack_decode.py executable_file [args]
#   2) Read log data from stdin: stack_decode.py -s executable_file
#
# In each case this script will add file and line information to any backtrace log
# lines found and echo back all non-Backtrace lines untouched.

import re
import subprocess
import sys


# Process the log output looking for stacktrace snippets, for each line found to
# contain backtrace output extract the address and call add2line to get the file
# and line information. Output appended to end of original backtrace line. Output
# any nonmatching lines unmodified. End when EOF received.
def decode_stacktrace_log(object_file, input_source, address_offset=0):
    traces = {}
    # Match something like:
    #     [backtrace] [bazel-out/local-dbg/bin/source/server/_virtual_includes/backtrace_lib/server/backtrace.h:84]
    backtrace_marker = "\[backtrace\] [^\s]+"
    # Match something like:
    #     ${backtrace_marker} #10: SYMBOL [0xADDR]
    # or:
    #     ${backtrace_marker} #10: [0xADDR]
    stackaddr_re = re.compile("%s #\d+:(?: .*)? \[(0x[0-9a-fA-F]+)\]$" % backtrace_marker)
    # Match something like:
    #     #10 0xLOCATION (BINARY+0xADDR)
    asan_re = re.compile(" *#\d+ *0x[0-9a-fA-F]+ *\([^+]*\+(0x[0-9a-fA-F]+)\)")

    try:
        while True:
            line = input_source.readline()
            if line == "":
                return  # EOF
            stackaddr_match = stackaddr_re.search(line)
            if not stackaddr_match:
                stackaddr_match = asan_re.search(line)
            if stackaddr_match:
                address = stackaddr_match.groups()[0]
                if address_offset != 0:
                    address = hex(int(address, 16) - address_offset)
                file_and_line_number = run_addr2line(object_file, address)
                file_and_line_number = trim_proc_cwd(file_and_line_number)
                if address_offset != 0:
                    sys.stdout.write("%s->[%s] %s" % (line.strip(), address, file_and_line_number))
                else:
                    sys.stdout.write("%s %s" % (line.strip(), file_and_line_number))
                continue
            else:
                # Pass through print all other log lines:
                sys.stdout.write(line)
    except KeyboardInterrupt:
        return


# Execute addr2line with a particular object file and input string of addresses
# to resolve, one per line.
#
# Returns list of result lines
def run_addr2line(obj_file, addr_to_resolve):
    return subprocess.check_output(["addr2line", "-Cpie", obj_file,
                                    addr_to_resolve]).decode('utf-8')


# Because of how bazel compiles, addr2line reports file names that begin with
# "/proc/self/cwd/" and sometimes even "/proc/self/cwd/./". This isn't particularly
# useful information, so trim it out and make a perfectly useful relative path.
def trim_proc_cwd(file_and_line_number):
    trim_regex = r'/proc/self/cwd/(\./)?'
    return re.sub(trim_regex, '', file_and_line_number)


# Execute pmap with a pid to calculate the addr offset
#
# Returns list of extended process memory information.
def run_pmap(pid):
    return subprocess.check_output(['pmap', '-qX', str(pid)]).decode('utf-8')[1:]


# Find the virtual address offset of the process. This may be needed due ASLR.
#
# Returns the virtual address offset as an integer, or 0 if unable to determine.
def find_address_offset(pid):
    try:
        proc_memory = run_pmap(pid)
        match = re.search(r'([a-f0-9]+)\s+r-xp', proc_memory)
        if match is None:
            return 0
        return int(match.group(1), 16)
    except (subprocess.CalledProcessError, PermissionError):
        return 0


# When setting the logging level to trace, it's possible that we'll bump
# into chars not accepted by the default encoding. It's fine to
# ignore these and keep going (instead of giving up and exiting
# while possibly bringing Envoy down).
def ignore_decoding_errors(io_wrapper):
    # Only avail since 3.7.
    # https://docs.python.org/3/library/io.html#io.TextIOWrapper.reconfigure
    if hasattr(io_wrapper, 'reconfigure'):
        try:
            io_wrapper.reconfigure(errors='ignore')
        except:
            pass

    return io_wrapper


if __name__ == "__main__":
    if len(sys.argv) > 2 and sys.argv[1] == '-s':
        decode_stacktrace_log(sys.argv[2], ignore_decoding_errors(sys.stdin))
        sys.exit(0)
    elif len(sys.argv) > 1:
        rununder = subprocess.Popen(
            sys.argv[1:], stdout=subprocess.PIPE, stderr=subprocess.STDOUT, universal_newlines=True)
        offset = find_address_offset(rununder.pid)
        decode_stacktrace_log(sys.argv[1], ignore_decoding_errors(rununder.stdout), offset)
        rununder.wait()
        sys.exit(rununder.returncode)  # Pass back test pass/fail result
    else:
        print("Usage (execute subprocess): stack_decode.py executable_file [additional args]")
        print("Usage (read from stdin): stack_decode.py -s executable_file")
        sys.exit(1)

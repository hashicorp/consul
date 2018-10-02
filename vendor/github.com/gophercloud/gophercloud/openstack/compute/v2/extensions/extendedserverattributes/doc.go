/*
Package extendedserverattributes provides the ability to extend a
server result with the extended usage information.

Example to Get an extended information:

  type serverAttributesExt struct {
    servers.Server
    extendedserverattributes.ServerAttributesExt
  }
  var serverWithAttributesExt serverAttributesExt

  err := servers.Get(computeClient, "d650a0ce-17c3-497d-961a-43c4af80998a").ExtractInto(&serverWithAttributesExt)
  if err != nil {
    panic(err)
  }

  fmt.Printf("%+v\n", serverWithAttributesExt)
*/
package extendedserverattributes

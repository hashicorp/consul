/*
Package imageimport enables management of images import and retrieval of the
Imageservice Import API information.

Example to Get an information about the Import API

	importInfo, err := imageimport.Get(imagesClient).Extract()
	if err != nil {
		panic(err)
	}

	fmt.Printf("%+v\n", importInfo)
*/
package imageimport

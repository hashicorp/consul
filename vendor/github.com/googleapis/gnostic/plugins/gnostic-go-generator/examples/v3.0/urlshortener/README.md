# urlshortener sample client

## Steps to run:

1. Generate the OpenAPI 3.0 description using `disco` (in the `gnostic/apps` directory).

        disco get urlshortener --openapi3
	
2. (optional) View the JSON OpenAPI 3.0 description.

        gnostic openapi3-urlshortener-v1.pb --json-out=-
	
3. Generate the urlshortener client.

        gnostic openapi3-urlshortener-v1.pb --go-client-out=urlshortener
	
4. Build the client.

        go install 
	
5. Download `client_secrets.json` from the Google Cloud Developer Console.

6. Run the client

        urlshortener
	

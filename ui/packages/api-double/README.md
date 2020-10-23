# api-double

api-double serving via HTTP or other means

See <https://github.com/hashicorp/consul-api-double/> for an example of an api-double.

'Templates' use simple native javascript template literals for very basic looping and basic logic for providing fake data.

## Usage

### CLI/Server

```bash
api-double --dir path/to/templates

# Flags

--dir : set the path to template files (default ./)
--seed : set the seed for faker to use
--port : set the port to serve from (default: 3000)

# ENV vars

HC_API_DOUBLE_PORT : default port to use
HC_API_DOUBLE_DIR : default path to use
HC_API_DOUBLE_SEED: default seed to use
```

### Browser/frontend only usage

TODO

## Wildcard templates

To provide a double for `/v1/health/service/:name`

Create a `/v1/health/service/_` template file. This will be used for `/v1/health/service/*`. Within the template the `*` will be in `location.segment(3)`

Further configuration will be provided by a `/v1/health/service/.config` file or similar as and when needed.

## Extra template helpers:

Right now very subject to change. But the idea is to keep them as minimal as possible and just rely on `faker`, plus helpers to get things you need for doing stuff like this (easy way to loop, access to url params and headers)

### HTTP properties

HTTP data is accessible via the http object using the following properties:

```
http.method
http.headers.*
http.body.*
http.cookies.*
```

### env(key, defaultValue)

Gets the 'environment' value specified by `key`, if it doesn't exist, use the
default value. 'environment' variables come from cookies by default, which
can be easily set using the browsers Web Inspector

### range(int)

Simple range function for creating loops

```javascript
[
    ${
        range(100000).map(
            item => {
                return `"service-${item}"`;
            }
        );
    }
]
// yields
[
    "service-1",
    ...,
    "service-100000"
]
```

### fake

Object containing access to various [`faker` functions](https://github.com/marak/Faker.js/#api-methods)

```javascript
[
    ${
        range(100000).map(
            item => {
                return `${fake.address.countryCode().toLowerCase()-${item}}`;
            }
        );
    }
]
// yields
[
    "it-1",
    ...,
    "de-100000"
]
```

### location.pathname

Reference to the current url

```javascript
// /v1/catalog/datacenters
[
    "${location.pathname}"
]
// yields
[
    "/v1/catalog/datacenters"
]
```

### location.search

This gives you a place to access queryParams `location.search.queryParamName`

### location.pathname.get(int)

Reference a 'segment' in the current url

```javascript
// /v1/catalog/datacenters
[
    "${location.pathname.get(1)}"
]
// yields
[
    "catalog"
]
```

### location.pathname.slice

### location.pathname.isDir




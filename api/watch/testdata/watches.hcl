watches = [
    {
        type = "key"
        key = "foo/bar/baz"
        handler_type = "http"
        http_handler_config  = {
            path = "http://localhost:8002/watch/key"
            method = "POST"
            timeout = "10s"
            header = {
                x-key = ["foo/bar/baz"]
            }
        }
    }
]
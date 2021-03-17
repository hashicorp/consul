route "dc" {
  path = "/:dc"

  route "services" {
  	path = "/services"

  	route "show" {
  	  path = "/:name"
  	  route "instances" {
  	    path = "/instances"
  	  }
  	  route "intentions" {
  	    path = "/intentions"
  	    route "edit" {
  	      path = "/:intention_id"
  	    }
  	    route "create" {
  	      path = "/create"
  	    }
  	  }
  	  route "topology" {
  	    path = "/topology"
  	  }
  	  route "services" {
  	    path = "/services"
  	  }
  	  route "upstreams" {
  	    path = "/upstreams"
  	  }
  	  route "routing" {
  	    path = "/routing"
  	  }
  	  route "tags" {
  	    path = "/tags"
  	  }
  	}

  	route "instance" {
  	  path = "/:name/instances/:node/:id"
  	  route "healthchecks" {
  	    path = "/health-checks"
  	  }
  	  route "upstreams" {
  	    path = "/upstreams"
  	  }
  	  route "exposedpaths" {
  	    path = "/exposed-paths"
  	  }
  	  route "addresses" {
  	    path = "/addresses"
  	  }
  	  route "metadata" {
  	    path = "/metadata"
  	  }
  	}
  	route "notfound" {
  	  path = "/:name/:node/:id"
  	}
  }

  route "nodes" {
  	path = "/nodes"
    route "show" {
  	  path = "/:name"
      route "healthchecks" {
  	    path = "/health-checks"
      }
      route "services" {
  	    path = "/service-instances"
      }
      route "rtt" {
  	    path = "/round-trip-time"
      }
      route "sessions" {
  	    path = "/lock-sessions"
      }
      route "metadata" {
  	    path = "/metadata"
      }
    }
  }

  route "intentions" {
  	path = "/intentions"
    route "edit" {
  	  path = "/:intention_id"
  	  abilities = ['read intentions']
    }
    route "create" {
  	  path = "/create"
  	  abilities = ['create intentions']
    }
  }

  route "kv" {
  	path = "/kv"
    route "folder" {
  	  path = "/*key"
    }
    route "edit" {
  	  path = "/*key/edit"
    }
    route "create" {
  	  path = "/*key/create"
  	  abilities = ['create kvs']
    }
    route "root-create" {
  	  path = "/create"
  	  abilities = ['create kvs']
    }
  }

  route "acls" {
  	path = "/acls"
  	abilities = ['read acls']
    route "edit" {
  	  path = "/:id"
    }
    route "create" {
  	  path = "/create"
  	  abilities = ['create acls']
    }

    route "policies" {
  	  path = "/policies"
  	  abilities = ['read policies']
      route "edit" {
  	    path = "/:id"
      }
      route "create" {
  	    path = "/create"
  	    abilities = ['create policies']
      }
    }

    route "roles" {
  	  path = "/roles"
  	  abilities = ['read roles']
      route "edit" {
  	    path = "/:id"
      }
      route "create" {
  	    path = "/create"
  	    abilities = ['create roles']
      }
    }

    route "tokens" {
  	  path = "/tokens"
  	  abilities = ['read tokens']
      route "edit" {
  	    path = "/:id"
      }
      route "create" {
  	    path = "/create"
  	    abilities = ['create tokens']
      }
    }

    route "auth-methods" {
  	  path = "/auth-methods"
  	  abilities = ['read auth-methods']
      route "show" {
  	    path = "/:id"
  	    route "auth-method" {
  	      path = "/auth-method"
  	    }
      }
    }

  }
}
route "index" {
  path = "/"
}

route "settings" {
  path = "/setting"
}

route "notfound" {
  path = "/*path"
}


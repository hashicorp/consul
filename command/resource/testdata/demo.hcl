ID {
  Type = gvk("demo.v2.Artist")
  Name = "korn"
  Tenancy {
    Namespace = "default"
    Partition = "default"
    PeerName = "local"
  }
}

Data {
  Name = "Korn"
  Genre = "GENRE_METAL"
}

Metadata = {
  "foo" = "bar"
}

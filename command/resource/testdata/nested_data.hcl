# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

ID {
  Type = gvk("demo.v2.Festival")
  Name = "woodstock"
}

Data {
  Genres = [
    "GENRE_JAZZ",
    "GENRE_FOLK",
    "GENRE_BLUES",
    "GENRE_ROCK",
  ]

  Artists = [
    {
      Name  = "Arlo Guthrie"
      Genre = "GENRE_FOLK"
    },
    {
      Name  = "Santana"
      Genre = "GENRE_BLUES"
    },
    {
      Name  = "Grateful Dead"
      Genre = "GENRE_ROCK"
    }
  ]
}

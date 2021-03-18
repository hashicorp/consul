  count = 2
resource "aws_instance" "example" {
  count = 2
  float = 1.1
  exp = 1e9

  ami           = "abc123"
  instance_type = "t2.micro"

  network_interface "here" {
  }

  one_liner { here = true }

  array = [1, "two", false, null]
  object = {prop: "value"}
  object2 = {"prop": 1}
  object3 = {'prop': 1.34}

  deepObject = {
    prop: {
      "prop": {
        'prop': [''],
        'prop2': ['']
      }
    }
  }

  lifecycle {
    // slash_create_before_destroy = true
    # hash_create_before_destroy = true
    create_after_destroy = false
    create = null
  }
}

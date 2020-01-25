behavior "regexp_issue_labeler" "panic_label" {
    regexp = "panic:"
    labels = ["crash"]
}

behavior "remove_labels_on_reply" "remove_stale" {
    labels = ["waiting-reply"]
}

poll "closed_issue_locker" "locker" {
  schedule             = "0 50 1 * * *"
  closed_for           = "720h" # 30 days
  max_issues           = 250
  sleep_between_issues = "1m"

  message = <<-EOF
    Hey there,

    This issue has been automatically locked because it is closed and there hasn't been any activity for at least _30_ days.

    If you are still experiencing problems, or still have questions, feel free to [open a new one](https://github.com/hashicorp/consul/issues/new) :+1:.
  EOF
}

poll "stale_issue_closer" "stale_closer" {
    schedule = "0 22 23 * * *"
    no_reply_in_last = "480h" # 20 days
    max_issues = 250
    sleep_between_issues = "1m"
    labels = ["waiting-reply"]
    message = <<-EOF
    Hey there,

    This issue has been automatically closed because there hasn't been any activity for at least _20_ days.

    If you are still experiencing problems, or still have questions, feel free to [open a new one](https://github.com/hashicorp/consul/issues/new) :+1:.
    EOF
}

poll "stale_issue_closer" "close_closer" {
    schedule = "0 50 2 * * *"
    no_reply_in_last = "1m" # hack to close issue with that label immediately.
    max_issues = 250
    sleep_between_issues = "1m"
    labels = ["close-issue"]
    message = <<-EOF
    Hey there,

    This issue has been automatically closed because it was labled with `close-issue`.

    If you are still experiencing problems, or still have questions, feel free to [open a new one](https://github.com/hashicorp/consul/issues/new) :+1:.
    EOF
}

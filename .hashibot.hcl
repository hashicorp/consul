behavior "regexp_issue_labeler" "panic_label" {
    regexp = "panic:"
    labels = ["crash"]
}

behavior "remove_labels_on_reply" "remove_stale" {
    labels = ["waiting-reply"]
}

poll "stale_issue_closer" "waiting_reply_closer" {
    schedule = "0 0 2 * * *"
    no_reply_in_last = "480h" # 20 days
    max_issues = 50
    sleep_between_issues = "5s"
    labels = ["waiting-reply-🤖"]
    message = <<-EOF
    Hey there,

    This issue has been automatically closed because there hasn't been any activity for at least _20_ days.

    If you are still experiencing problems, or still have questions, feel free to [open a new one](https://github.com/hashicorp/consul/issues/new) :+1:.
    EOF
}

poll "stale_issue_closer" "old_issue_closer" {
    schedule = "0 0 3 * * *"
    no_reply_in_last = "1s"
    max_issues = 100
    sleep_between_issues = "5s"
    labels = ["close-old-issue-🤖"]
    message = <<-EOF
    Hey there,

    This issue was reported on a version of Consul that is quite old. Based on the volume of changes since that version we encourage you to try reproducing with the latest version of Consul. If you feel the issue is critical and on an older version of Consul, or still exists in the current version, feel free to [open a new issue](https://github.com/hashicorp/consul/issues/new) :+1:.
    EOF
}

# poll "closed_issue_locker" "locker" {
#   schedule             = "0 0 4 * * *"
#   closed_for           = "720h" # 30 days
#   max_issues           = 250
#   sleep_between_issues = "1m"
# 
#   message = <<-EOF
#     Hey there,
# 
#     This issue has been automatically locked because it is closed and there hasn't been any activity for at least _30_ days.
# 
#     If you are still experiencing problems, or still have questions, feel free to [open a new one](https://github.com/hashicorp/consul/issues/new) :+1:.
#   EOF
# }

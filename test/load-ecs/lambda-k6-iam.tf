data "aws_iam_policy_document" "lambda-assume" {
  statement {
    effect = "Allow"
    actions = [
      "sts:AssumeRole",
    ]
    principals {
      identifiers = ["lambda.amazonaws.com"]
      type        = "Service"
    }
  }
}

resource "aws_iam_role" "lambda-assume" {
  name               = "${var.cluster_name}LambdaK6Assume"
  assume_role_policy = data.aws_iam_policy_document.lambda-assume.json
}

data "aws_iam_policy_document" "lambda_basic_execution" {
  statement {
    sid = "AWSLambdaBasicExecutionRole"

    actions = [
      "ec2:CreateNetworkInterface",
      "ec2:DescribeNetworkInterfaces",
      "ec2:DeleteNetworkInterface",
    ]

    resources = ["*"]
  }
}

resource "aws_iam_policy" "lambda_basic_execution" {
  name   = "${var.cluster_name}LambdaK6Network"
  policy = data.aws_iam_policy_document.lambda_basic_execution.json
}

resource "aws_iam_role_policy_attachment" "lambda_basic_execution" {
  role       = aws_iam_role.lambda-assume.name
  policy_arn = aws_iam_policy.lambda_basic_execution.arn
}

resource "aws_iam_policy" "lambda_logging" {
  name        = "lambda_logging"
  path        = "/"
  description = "IAM policy for logging from a lambda"

  policy = jsonencode({
    Version: "2012-10-17",
    Statement: [
      {
        Action: [
          "logs:CreateLogGroup",
          "logs:CreateLogStream",
          "logs:PutLogEvents"
        ],
        Resource: "arn:aws:logs:*:*:*",
        Effect: "Allow"
      }
    ]
  })
}

resource "aws_iam_role_policy_attachment" "lambda_logs" {
  role       = aws_iam_role.lambda-assume.name
  policy_arn = aws_iam_policy.lambda_logging.arn
}


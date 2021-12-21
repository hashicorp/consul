---
name: Consul docs day task
about: This is a HashiCorp internal documentation task for the purpose of the Consul docs day event.
labels: ['type/docs', 'placeholder']
assignees: 
    - karl-cardenas-coding
    - jkirschner-hashicorp
    - trujillo-adam
---

body:
  - type: markdown
    attributes:
      value: |
        Thanks for taking the time to fill out this Consul docs event task request!
 - type: dropdown
    id: Category
    attributes:
      label: Category
      description: What category is this task for?
      options:
        - Add/Improve Examples
        - Structural Improvements and Cleanup
        - Visual Aids
    validations:
      required: true
  - type: input
    id: Short Name
    attributes:
      label: Short Name
      description: Please provide a short name for this task. 
      placeholder: ex. PKI Overview
    validations:
      required: true
  - type: textarea
    id: Description
    attributes:
      label: Description
      description: Please provide details about the task?
      placeholder: What I'll change and why this is important
    validations:
      required: true

  - type: textarea
    id: link
    attributes:
      label: Documentation page link(s)
      description: Please provide link(s) to the documentation page(s) to be modified, if applicable
      placeholder: ex: https://www.consul.io/docs/connect/gateways#gateways
    validations:
      required: true
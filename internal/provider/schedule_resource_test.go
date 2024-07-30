package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccOrderResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testProviderConfig + `
resource "temporal_schedule" "example" {
  name             = "Example Schedule"
  is_paused        = true
  pause_on_failure = true
  overlap_policy   = "skip"
  catchup_window   = "3h"

  action {
    workflow_id     = "exampleWorkflow-dailyRun"
    workflow_type   = "exampleWorkflow"
    task_queue_name = "example-task-queue"
    input_payload = jsonencode({
      myVar = "abc"
    })
  }

  spec {
    interval {
      every  = "1d"
      offset = "1h"
    }
  }
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("temporal_schedule.example", "name", "Example Schedule"),
					resource.TestCheckResourceAttr("temporal_schedule.example", "is_paused", "true"),
					resource.TestCheckResourceAttr("temporal_schedule.example", "pause_on_failure", "true"),
					resource.TestCheckResourceAttr("temporal_schedule.example", "overlap_policy", "skip"),
					resource.TestCheckResourceAttr("temporal_schedule.example", "catchup_window", "3h"),
					resource.TestCheckResourceAttr("temporal_schedule.example", "action.workflow_id", "exampleWorkflow-dailyRun"),
					resource.TestCheckResourceAttr("temporal_schedule.example", "action.workflow_type", "exampleWorkflow"),
					resource.TestCheckResourceAttr("temporal_schedule.example", "action.task_queue_name", "example-task-queue"),
					resource.TestCheckResourceAttr("temporal_schedule.example", "action.input_payload", "{\"myVar\":\"abc\"}"),
					resource.TestCheckResourceAttr("temporal_schedule.example", "spec.interval.0.every", "1d"),
					resource.TestCheckResourceAttr("temporal_schedule.example", "spec.interval.0.offset", "1h"),
				),
			},
			// Create and Read testing - required fields only
			{
				Config: testProviderConfig + `
resource "temporal_schedule" "example" {
  name             = "Example Schedule"

  action {
    workflow_id     = "exampleWorkflow-dailyRun"
    workflow_type   = "exampleWorkflow"
    task_queue_name = "example-task-queue"
  }

  spec {
    interval {
      every  = "1d"
    }
  }
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("temporal_schedule.example", "name", "Example Schedule"),
					resource.TestCheckResourceAttr("temporal_schedule.example", "is_paused", "false"),
					resource.TestCheckResourceAttr("temporal_schedule.example", "pause_on_failure", "false"),
					resource.TestCheckResourceAttr("temporal_schedule.example", "overlap_policy", "skip"),
					resource.TestCheckResourceAttr("temporal_schedule.example", "catchup_window", "365d"),
					resource.TestCheckResourceAttr("temporal_schedule.example", "action.workflow_id", "exampleWorkflow-dailyRun"),
					resource.TestCheckResourceAttr("temporal_schedule.example", "action.workflow_type", "exampleWorkflow"),
					resource.TestCheckResourceAttr("temporal_schedule.example", "action.task_queue_name", "example-task-queue"),
					resource.TestCheckNoResourceAttr("temporal_schedule.example", "action.input_payload"),
					resource.TestCheckResourceAttr("temporal_schedule.example", "spec.interval.0.every", "1d"),
					resource.TestCheckResourceAttr("temporal_schedule.example", "spec.interval.0.offset", "0s"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

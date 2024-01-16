# cloudwatch metrics

This package provides a generalized mechanism to retrieve metrics from cloudwatch for a Nullstone workspace.

Here is a high-level workflow of how the `Getter` struct retrieves metrics for any cloudwatch metrics:
1. Get `metrics_reader` (aws user) and `metrics_mappings` from workspace outputs.
2. Calculate a period for querying metrics that will produce approximately 60 datapoints per series. (This provides a nice balance of performance and granularity.)
3. Run an API call to `GetMetricData` using `metrics_mappings` from workspace outputs to query all metric data.
4. Map the results back to a standardized metrics dataset. (This dataset is consistent across all metric sources.)

## `metrics_mappings`

This implementation allows each Terraform module to define a set of metrics that can be retrieved. This has 2 major benefits:
1. Metrics can be added/removed from the dashboard based on capabilities. (Example: Add load balancer metrics when a load balancer is attached)
2. Many metric queries rely on specific information (i.e. names, IDs, ARNs) that are readily available during a Terraform plan.
3. This provides an extensible platform where any platform engineer can add their own metrics. 

### Format

The following format is used to emit `metrics_mappings` from a Terraform workspace.
```hcl
locals {
  metrics_mappings = [
    // Each group is plotted as a single graph
    {
      name = "" // Title of graph
      type = "usage" // Type of graph (e.g. "usage", "invocations", "duration")
      unit = "" // Added to graph to denote unit of measurement
      
      mappings = {
        // Each mapping represents a series of data
        // They are each plotted according to the graph type
        // metric_id is critical to showing graph types properly
        cpu_reserved = {
          account_id  = "<aws account id>"
          stat        = "Average" // Average, Minimum, Maximum, Sum
          namespace   = "ECS/ContainerInsights" // Cloudwatch Metrics Namespace
          metric_name = "CpuReserved" // Cloudwatch Metric Name
          dimensions  = {
            // Dimensions are specific to each metric being queried
            // ECS/ContainerInsights: https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/Container-Insights-metrics-ECS.html
            "ClusterName" = "<cluster-name>"
            "ServiceName" = "<service-name>"
          }
        },
        // ...more mappings
      }
    },
    // ...more groups
  ]
}
```

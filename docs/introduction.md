# The ClusterCockpit Project

The ClusterCockpit project is a joined project of computing centers in Europe to set up a cluster monitoring stack for small to mid-sized computing centers under the lead of NHR@FAU.

# The ClusterCockpit Stack

In cluster environment, there are commonly a lot of systems dedicated for computation, backend servers for file systems and frontend servers for the user interaction and cluster control. The ClusterCockpit Stack is mainly used for monitoring the compute systems with some interaction to the frontend servers. It consists of multiple components:

- cc-metric-collector: Monitor resource usage on the compute systems
- cc-metric-store: In-memory database
- cc-backend & cc-frontend: The web-based visualizer

# CC Metric Collector

The CC Metric Collector project was started to provide a useful set of metrics for HPC and data science related compute systems. It runs as a system daemon and gathers system data periodically to forward the metrics to one or more databases. One of the provided backends can be used for the cc-metric-store but many others exist like InfluxDB time-series databases, the Ganglia Monitoring System or the Prometheus Monitoring System.

The data is gathered by so-called "Collectors", forwarded to an internal router for on-the-fly manipulation (tagging, aggregation, ...) which pushes the metrics to the different metric writers called "Sinks". There is a forth component, the "Receivers", which receive data through some networking system like a HTTP server at any time.

# CC Metric Store
The CC Metric Store is a data management system with short-term in-memory and long-term file-base metric storage.

# CC Backend and CC Frontend
The CC Backend and Frontend form together the web interface for ClusterCockpit.
package jobs

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"

	"github.com/databrickslabs/terraform-provider-databricks/clusters"
	"github.com/databrickslabs/terraform-provider-databricks/common"
	"github.com/databrickslabs/terraform-provider-databricks/libraries"
)

// NotebookTask contains the information for notebook jobs
type NotebookTask struct {
	NotebookPath   string            `json:"notebook_path"`
	BaseParameters map[string]string `json:"base_parameters,omitempty"`
}

// SparkPythonTask contains the information for python jobs
type SparkPythonTask struct {
	PythonFile string   `json:"python_file"`
	Parameters []string `json:"parameters,omitempty"`
}

// SparkJarTask contains the information for jar jobs
type SparkJarTask struct {
	JarURI        string   `json:"jar_uri,omitempty"`
	MainClassName string   `json:"main_class_name,omitempty"`
	Parameters    []string `json:"parameters,omitempty"`
}

// SparkSubmitTask contains the information for spark submit jobs
type SparkSubmitTask struct {
	Parameters []string `json:"parameters,omitempty"`
}

// PythonWheelTask contains the information for python wheel jobs
type PythonWheelTask struct {
	EntryPoint      string            `json:"entry_point,omitempty"`
	PackageName     string            `json:"package_name,omitempty"`
	Parameters      []string          `json:"parameters,omitempty"`
	NamedParameters map[string]string `json:"named_parameters,omitempty"`
}

// PipelineTask contains the information for pipeline jobs
type PipelineTask struct {
	PipelineID string `json:"pipeline_id"`
}

// EmailNotifications contains the information for email notifications after job completion
type EmailNotifications struct {
	OnStart               []string `json:"on_start,omitempty"`
	OnSuccess             []string `json:"on_success,omitempty"`
	OnFailure             []string `json:"on_failure,omitempty"`
	NoAlertForSkippedRuns bool     `json:"no_alert_for_skipped_runs,omitempty"`
}

// CronSchedule contains the information for the quartz cron expression
type CronSchedule struct {
	QuartzCronExpression string `json:"quartz_cron_expression"`
	TimezoneID           string `json:"timezone_id"`
	PauseStatus          string `json:"pause_status,omitempty" tf:"computed"`
}

type TaskDependency struct {
	TaskKey string `json:"task_key,omitempty"`
}

type JobTaskSettings struct {
	TaskKey     string           `json:"task_key,omitempty"`
	Description string           `json:"description,omitempty"`
	DependsOn   []TaskDependency `json:"depends_on,omitempty"`

	ExistingClusterID      string              `json:"existing_cluster_id,omitempty" tf:"group:cluster_type"`
	NewCluster             *clusters.Cluster   `json:"new_cluster,omitempty" tf:"group:cluster_type"`
	Libraries              []libraries.Library `json:"libraries,omitempty" tf:"slice_set,alias:library"`
	NotebookTask           *NotebookTask       `json:"notebook_task,omitempty" tf:"group:task_type"`
	SparkJarTask           *SparkJarTask       `json:"spark_jar_task,omitempty" tf:"group:task_type"`
	SparkPythonTask        *SparkPythonTask    `json:"spark_python_task,omitempty" tf:"group:task_type"`
	SparkSubmitTask        *SparkSubmitTask    `json:"spark_submit_task,omitempty" tf:"group:task_type"`
	PipelineTask           *PipelineTask       `json:"pipeline_task,omitempty" tf:"group:task_type"`
	PythonWheelTask        *PythonWheelTask    `json:"python_wheel_task,omitempty" tf:"group:task_type"`
	EmailNotifications     *EmailNotifications `json:"email_notifications,omitempty" tf:"suppress_diff"`
	TimeoutSeconds         int32               `json:"timeout_seconds,omitempty"`
	MaxRetries             int32               `json:"max_retries,omitempty"`
	MinRetryIntervalMillis int32               `json:"min_retry_interval_millis,omitempty"`
	RetryOnTimeout         bool                `json:"retry_on_timeout,omitempty" tf:"computed"`
}

// JobSettings contains the information for configuring a job on databricks
type JobSettings struct {
	Name string `json:"name,omitempty" tf:"default:Untitled"`

	// BEGIN Jobs API 2.0
	ExistingClusterID      string              `json:"existing_cluster_id,omitempty" tf:"group:cluster_type"`
	NewCluster             *clusters.Cluster   `json:"new_cluster,omitempty" tf:"group:cluster_type"`
	NotebookTask           *NotebookTask       `json:"notebook_task,omitempty" tf:"group:task_type"`
	SparkJarTask           *SparkJarTask       `json:"spark_jar_task,omitempty" tf:"group:task_type"`
	SparkPythonTask        *SparkPythonTask    `json:"spark_python_task,omitempty" tf:"group:task_type"`
	SparkSubmitTask        *SparkSubmitTask    `json:"spark_submit_task,omitempty" tf:"group:task_type"`
	PipelineTask           *PipelineTask       `json:"pipeline_task,omitempty" tf:"group:task_type"`
	PythonWheelTask        *PythonWheelTask    `json:"python_wheel_task,omitempty" tf:"group:task_type"`
	Libraries              []libraries.Library `json:"libraries,omitempty" tf:"slice_set,alias:library"`
	TimeoutSeconds         int32               `json:"timeout_seconds,omitempty"`
	MaxRetries             int32               `json:"max_retries,omitempty"`
	MinRetryIntervalMillis int32               `json:"min_retry_interval_millis,omitempty"`
	RetryOnTimeout         bool                `json:"retry_on_timeout,omitempty"`
	// END Jobs API 2.0

	// BEGIN Jobs API 2.1
	Tasks  []JobTaskSettings `json:"tasks,omitempty" tf:"alias:task"`
	Format string            `json:"format,omitempty" tf:"computed"`
	// END Jobs API 2.1

	Schedule           *CronSchedule       `json:"schedule,omitempty"`
	MaxConcurrentRuns  int32               `json:"max_concurrent_runs,omitempty"`
	EmailNotifications *EmailNotifications `json:"email_notifications,omitempty" tf:"suppress_diff"`
}

func (js *JobSettings) isMultiTask() bool {
	return js.Format == "MULTI_TASK" || len(js.Tasks) > 0
}

func (js *JobSettings) sortTasksByKey() {
	sort.Slice(js.Tasks, func(i, j int) bool {
		return js.Tasks[i].TaskKey < js.Tasks[j].TaskKey
	})
}

// JobList returns a list of all jobs
type JobList struct {
	Jobs []Job `json:"jobs"`
}

// Job contains the information when using a GET request from the Databricks Jobs api
type Job struct {
	JobID           int64        `json:"job_id,omitempty"`
	CreatorUserName string       `json:"creator_user_name,omitempty"`
	Settings        *JobSettings `json:"settings,omitempty"`
	CreatedTime     int64        `json:"created_time,omitempty"`
}

// ID returns job id as string
func (j Job) ID() string {
	return fmt.Sprintf("%d", j.JobID)
}

// RunParameters used to pass params to tasks
type RunParameters struct {
	// a shortcut field to reuse this type for RunNow
	JobID int64 `json:"job_id,omitempty"`

	NotebookParams    map[string]string `json:"notebook_params,omitempty"`
	JarParams         []string          `json:"jar_params,omitempty"`
	PythonParams      []string          `json:"python_params,omitempty"`
	SparkSubmitParams []string          `json:"spark_submit_params,omitempty"`
}

// RunState of the job
type RunState struct {
	ResultState    string `json:"result_state,omitempty"`
	LifeCycleState string `json:"life_cycle_state,omitempty"`
	StateMessage   string `json:"state_message,omitempty"`
}

// JobRun is a simplified representation of corresponding entity
type JobRun struct {
	JobID       int64    `json:"job_id"`
	RunID       int64    `json:"run_id"`
	NumberInJob int64    `json:"number_in_job"`
	StartTime   int64    `json:"start_time,omitempty"`
	State       RunState `json:"state"`
	Trigger     string   `json:"trigger,omitempty"`
	RuntType    string   `json:"run_type,omitempty"`

	OverridingParameters RunParameters `json:"overriding_parameters,omitempty"`
}

// JobRunsListRequest used to do what it sounds like
type JobRunsListRequest struct {
	JobID         int64 `url:"job_id,omitempty"`
	ActiveOnly    bool  `url:"active_only,omitempty"`
	CompletedOnly bool  `url:"completed_only,omitempty"`
	Offset        int32 `url:"offset,omitempty"`
	Limit         int32 `url:"limit,omitempty"`
}

// JobRunsList returns a page of job runs
type JobRunsList struct {
	Runs    []JobRun `json:"runs"`
	HasMore bool     `json:"has_more"`
}

// UpdateJobRequest used to do what it sounds like
type UpdateJobRequest struct {
	JobID       int64        `json:"job_id,omitempty" url:"job_id,omitempty"`
	NewSettings *JobSettings `json:"new_settings,omitempty" url:"new_settings,omitempty"`
}

// NewJobsAPI creates JobsAPI instance from provider meta
func NewJobsAPI(ctx context.Context, m interface{}) JobsAPI {
	client := m.(*common.DatabricksClient)
	return JobsAPI{client, ctx}
}

// JobsAPI exposes the Jobs API
type JobsAPI struct {
	client  *common.DatabricksClient
	context context.Context
}

// List all jobs
func (a JobsAPI) List() (l JobList, err error) {
	err = a.client.Get(a.context, "/jobs/list", nil, &l)
	return
}

// RunsList returns a job runs list
func (a JobsAPI) RunsList(r JobRunsListRequest) (jrl JobRunsList, err error) {
	err = a.client.Get(a.context, "/jobs/runs/list", r, &jrl)
	return
}

// RunsCancel cancels job run and waits till it's finished
func (a JobsAPI) RunsCancel(runID int64, timeout time.Duration) error {
	var response interface{}
	err := a.client.Post(a.context, "/jobs/runs/cancel", map[string]interface{}{
		"run_id": runID,
	}, &response)
	if err != nil {
		return err
	}
	return a.waitForRunState(runID, "TERMINATED", timeout)
}

func (a JobsAPI) waitForRunState(runID int64, desiredState string, timeout time.Duration) error {
	return resource.RetryContext(a.context, timeout, func() *resource.RetryError {
		jobRun, err := a.RunsGet(runID)
		if err != nil {
			return resource.NonRetryableError(
				fmt.Errorf("cannot get job %s: %v", desiredState, err))
		}
		state := jobRun.State
		if state.LifeCycleState == desiredState {
			return nil
		}
		if state.LifeCycleState == "INTERNAL_ERROR" {
			return resource.NonRetryableError(
				fmt.Errorf("cannot get job %s: %s",
					desiredState, state.StateMessage))
		}
		return resource.RetryableError(
			fmt.Errorf("run is %s: %s",
				state.LifeCycleState,
				state.StateMessage))
	})
}

// RunNow triggers the job and returns a run ID
func (a JobsAPI) RunNow(jobID int64) (int64, error) {
	var jr JobRun
	err := a.client.Post(a.context, "/jobs/run-now", RunParameters{
		JobID: jobID,
	}, &jr)
	return jr.RunID, err
}

// RunsGet to retrieve information about the run
func (a JobsAPI) RunsGet(runID int64) (JobRun, error) {
	var jr JobRun
	err := a.client.Get(a.context, "/jobs/runs/get", map[string]interface{}{
		"run_id": runID,
	}, &jr)
	return jr, err
}

func (a JobsAPI) Start(jobID int64, timeout time.Duration) error {
	runID, err := a.RunNow(jobID)
	if err != nil {
		return fmt.Errorf("cannot start job run: %v", err)
	}
	return a.waitForRunState(runID, "RUNNING", timeout)
}

func (a JobsAPI) Restart(id string, timeout time.Duration) error {
	jobID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return err
	}
	runs, err := a.RunsList(JobRunsListRequest{JobID: jobID, ActiveOnly: true})
	if err != nil {
		return err
	}
	if len(runs.Runs) == 0 {
		// nothing to cancel
		return a.Start(jobID, timeout)
	}
	if len(runs.Runs) > 1 {
		return fmt.Errorf("`always_running` must be specified only with "+
			"`max_concurrent_runs = 1`. There are %d active runs", len(runs.Runs))
	}
	if len(runs.Runs) == 1 {
		activeRun := runs.Runs[0]
		err = a.RunsCancel(activeRun.RunID, timeout)
		if err != nil {
			return fmt.Errorf("cannot cancel run %d: %v", activeRun.RunID, err)
		}
	}
	return a.Start(jobID, timeout)
}

// Create creates a job on the workspace given the job settings
func (a JobsAPI) Create(jobSettings JobSettings) (Job, error) {
	var job Job
	jobSettings.sortTasksByKey()
	err := a.client.Post(a.context, "/jobs/create", jobSettings, &job)
	return job, err
}

// Update updates a job given the id and a new set of job settings
func (a JobsAPI) Update(id string, jobSettings JobSettings) error {
	jobID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return err
	}
	return wrapMissingJobError(a.client.Post(a.context, "/jobs/reset", UpdateJobRequest{
		JobID:       jobID,
		NewSettings: &jobSettings,
	}, nil), id)
}

// Read returns the job object with all the attributes
func (a JobsAPI) Read(id string) (job Job, err error) {
	jobID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return
	}
	err = wrapMissingJobError(a.client.Get(a.context, "/jobs/get", map[string]int64{
		"job_id": jobID,
	}, &job), id)
	if job.Settings != nil {
		job.Settings.sortTasksByKey()
	}
	return
}

// Delete deletes the job given a job id
func (a JobsAPI) Delete(id string) error {
	jobID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return err
	}
	return wrapMissingJobError(a.client.Post(a.context, "/jobs/delete", map[string]int64{
		"job_id": jobID,
	}, nil), id)
}

func wrapMissingJobError(err error, id string) error {
	if err == nil {
		return nil
	}
	apiErr, ok := err.(common.APIError)
	if !ok {
		return err
	}
	if apiErr.IsMissing() {
		return err
	}
	// fix non-compliant error code
	if strings.Contains(apiErr.Message,
		fmt.Sprintf("Job %s does not exist.", id)) {
		apiErr.StatusCode = 404
		return apiErr
	}
	return err
}

func jobSettingsSchema(s *map[string]*schema.Schema, prefix string) {
	if p, err := common.SchemaPath(*s, "new_cluster", "num_workers"); err == nil {
		p.Optional = true
		p.Default = 0
		p.Type = schema.TypeInt
		p.ValidateDiagFunc = validation.ToDiagFunc(validation.IntAtLeast(0))
		p.Required = false
	}
	if v, err := common.SchemaPath(*s, "new_cluster", "spark_conf"); err == nil {
		reSize := common.MustCompileKeyRE(prefix + "new_cluster.0.spark_conf.%")
		reConf := common.MustCompileKeyRE(prefix + "new_cluster.0.spark_conf.spark.databricks.delta.preview.enabled")
		v.DiffSuppressFunc = func(k, old, new string, d *schema.ResourceData) bool {
			isPossiblyLegacyConfig := reSize.Match([]byte(k)) && old == "1" && new == "0"
			isLegacyConfig := reConf.Match([]byte(k))
			if isPossiblyLegacyConfig || isLegacyConfig {
				log.Printf("[DEBUG] Suppressing diff for k=%#v old=%#v new=%#v", k, old, new)
				return true
			}
			return false
		}
	}
}

var jobSchema = common.StructToSchema(JobSettings{},
	func(s map[string]*schema.Schema) map[string]*schema.Schema {
		jobSettingsSchema(&s, "")
		jobSettingsSchema(&s["task"].Elem.(*schema.Resource).Schema, "task.0.")
		if p, err := common.SchemaPath(s, "schedule", "pause_status"); err == nil {
			p.ValidateFunc = validation.StringInSlice([]string{"PAUSED", "UNPAUSED"}, false)
		}
		s["max_concurrent_runs"].ValidateDiagFunc = validation.ToDiagFunc(validation.IntAtLeast(1))
		s["max_concurrent_runs"].Default = 1
		s["url"] = &schema.Schema{
			Type:     schema.TypeString,
			Computed: true,
		}
		s["always_running"] = &schema.Schema{
			Optional: true,
			Default:  false,
			Type:     schema.TypeBool,
		}
		return s
	})

func ResourceJob() *schema.Resource {
	getReadCtx := func(ctx context.Context, d *schema.ResourceData) context.Context {
		var js JobSettings
		common.DataToStructPointer(d, jobSchema, &js)
		if js.isMultiTask() {
			return context.WithValue(ctx, common.Api, common.API_2_1)
		}
		return ctx
	}
	return common.Resource{
		Schema:        jobSchema,
		SchemaVersion: 2,
		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(clusters.DefaultProvisionTimeout),
			Update: schema.DefaultTimeout(clusters.DefaultProvisionTimeout),
		},
		CustomizeDiff: func(ctx context.Context, d *schema.ResourceDiff, m interface{}) error {
			var js JobSettings
			common.DiffToStructPointer(d, jobSchema, &js)
			alwaysRunning := d.Get("always_running").(bool)
			if alwaysRunning && js.MaxConcurrentRuns > 1 {
				return fmt.Errorf("`always_running` must be specified only with `max_concurrent_runs = 1`")
			}
			for _, task := range js.Tasks {
				if task.NewCluster == nil {
					continue
				}
				if err := task.NewCluster.Validate(); err != nil {
					return fmt.Errorf("task %s invalid: %w", task.TaskKey, err)
				}
			}
			if js.NewCluster != nil {
				if err := js.NewCluster.Validate(); err != nil {
					return fmt.Errorf("invalid job cluster: %w", err)
				}
			}
			return nil
		},
		Create: func(ctx context.Context, d *schema.ResourceData, c *common.DatabricksClient) error {
			var js JobSettings
			common.DataToStructPointer(d, jobSchema, &js)
			if js.isMultiTask() {
				ctx = context.WithValue(ctx, common.Api, common.API_2_1)
			}
			jobsAPI := NewJobsAPI(ctx, c)
			job, err := jobsAPI.Create(js)
			if err != nil {
				return err
			}
			d.SetId(job.ID())
			if d.Get("always_running").(bool) {
				return jobsAPI.Start(job.JobID, d.Timeout(schema.TimeoutCreate))
			}
			return nil
		},
		Read: func(ctx context.Context, d *schema.ResourceData, c *common.DatabricksClient) error {
			ctx = getReadCtx(ctx, d)
			job, err := NewJobsAPI(ctx, c).Read(d.Id())
			if err != nil {
				return err
			}
			d.Set("url", c.FormatURL("#job/", d.Id()))
			return common.StructToData(*job.Settings, jobSchema, d)
		},
		Update: func(ctx context.Context, d *schema.ResourceData, c *common.DatabricksClient) error {
			var js JobSettings
			common.DataToStructPointer(d, jobSchema, &js)
			if js.isMultiTask() {
				ctx = context.WithValue(ctx, common.Api, common.API_2_1)
			}
			jobsAPI := NewJobsAPI(ctx, c)
			err := jobsAPI.Update(d.Id(), js)
			if err != nil {
				return err
			}
			if d.Get("always_running").(bool) {
				return jobsAPI.Restart(d.Id(), d.Timeout(schema.TimeoutUpdate))
			}
			return nil
		},
		Delete: func(ctx context.Context, d *schema.ResourceData, c *common.DatabricksClient) error {
			ctx = getReadCtx(ctx, d)
			return NewJobsAPI(ctx, c).Delete(d.Id())
		},
	}.ToResource()
}

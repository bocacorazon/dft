I want to build a set of tools to support my development process. 
I need to orchestrate a mixture of agents and tools chained in workflows that will coordinate the process at different levels.
I have developed different versions of this in the past few months -like [skrunner](https://github.com/bocacorazon/skrunner), and early but successful attempt at orchestrating the execution of [spec-kit](https://github.com/github/spec-kit) flows. My intention is to heavily use spec-driven development for my work.

My process will consist of three major phases, all of which I would like to support with this new toolkit.

# intent
In this phase I would like to have a set of agents to help me describe my intention for a project, feature, product increment, bug fix, etc. This is what is commonly referred to as 'PRD'
The output of this phase is a structured, validated package consisting of a description of the software artifact, additional feature that I want, but crucially, with a comprehensive set of acceptance criteria that I will use in later phases to validate that the output produce is the right one.
I will refer to this output the 'demand package'.
## solution design
In this phase I will need a set of agents to:
* create architecture blueprints for the solution
* convert the acceptance criteria to be used by the eval engine to automatically produce a verdict  on the output of the build engine
* create a Work Breakdown Structure ('WBS') set of artifacts:
	* A document describing the nature of the work and the different specs in which we will break down the project
	* A machine-readable structured doc (json or yaml) that will be the input to the Orchestrator to build the desired artifacts
* since not all demand packages will require the same level of structure to the dev process, a 'lane selector' that will assign different workflows to a particular demand package ('spec', 'streamlined', 'recursive', etc.) 
## orchestration
In this phase we conduct the actual construction of the requested artifacts. 
This should be a set of 'flows', a DAG of 'steps'. Here's a first draft of the [dsl](docs/flow-dsl.yaml).

My vision is that when a wbs is submitted to the orchestation layer, it will start two parallel threads:
- Build thread, that will execute the flow steps specific to this WBS
- an Eval thread with an *adversarial* relationship to the Build, that will generate all sort of ways to evaluate Build's output using different strategies

### flow execution
We should have a pluggable flow execution engine. For example:
-  a local one, where work parallelism is handled by different threads, 
- a Docker engine, where we would package a flow for execution in a container,
- a Kubernetes-based one
- a GH Actions based one
- etc.
I only want to focus on the local one for now.


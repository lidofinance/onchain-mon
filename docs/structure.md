More full information about how to struct your go.app you can find here https://github.com/golang-standards/project-layout

my-awesome-go-project
```text
|
└───api
|  
└───assets
|      |
|      └───images
|      |      image.jpg    
|      |
|      └───logs
|             log.txt
|
└───build
|
|
└───cmd
│   |
│   └───web-server
│   |       main.go
│   └───daemon
|   |       main.go
|   └───worker    
|           main.go
|
└───configs or etc
|     some.yaml  
|
└───deployments или deploy
|
└───docs
|      
└───examples
|      sample_some_request.http
|
|
└───internal
|        |
|        |
|        └───api
|        |    └───handler
|        |          └───handler
|        |                 handler.go
|        |                 handler_test.go
|        |
|        └───app
|        |    └───web-server
|        |    |      some_file.go        
|        |    │     
|        |    └───daemon
|        |    |       some_file.go
|        |    |
|        |    └───worker
|        |           some_file.go
|        |
|        └───http
|        |    |
|        |    └───middleware
|        |    |         check_some_permission.go  
|        |    |         check_some_permission_test.go
|        |    |
|        |    └───some_name_handler
|        |                handler.go
|        |                handler_test.go
|        |
|        └───grpc
|        |    |
|        |    └───some_name_grpc_handler     
|        |                handler.go
|        |                handler_test.go
|        |
|        └───pkg
|        |    │   
|        |    └───domain
|        |           |
|        |           └───mocks (generates automatically by mockery)
|        |           |      Repository.go
|        |           |      Usecase.go
|        |           └───entity
|        |           |     your_model_name.go
|        |           |
|        |           └───usecase
|        |           |      usecase.go
|        |           |      usecase_test.go
|        |           |
|        |           └───repository
|        |           |      repository.go
|        |           |      repository_test.go
|        |           |
|        |           | repository.go (interface для repository)
|        |           | usecase.go    (interface для usecase)
|        |           
|        └───utils
|              |
|              └───pointers
|              |      pointers.go
|              |      pointers_test.go
|              └───slices
|              |      slices.go
|              |      slices_test.go
|              └───strings
|                     strings.go
|                     strings_test.go               
|
|
└───pkg (If the repository is a library, then external projects, will able to import packages from this folder)
|
|
└───scripts
|     some_bash.sh
|
└───test (For integrations, smoke and other tests and test's data)       
|
|
└───tools (could use code from internal/*, pkg/*)
|     |
|     └───migrator_tools 
|     |
|     └───some_linter_tool
|     |   
|     └───and_3third_party_app_for_tooling_purposes
|
|
|  
└───web
|    └───react_spa_app
|    |
|    └───vue_spa_app
|    |
|    └───flutter_app
|    |
|    └───html_templates (twig or blade for example)
|
└───website
|     |
|     └───one page app aka git hub pages or etc
|
|
└───vendor
|
|   app.toml
│   README.md
│   robots.txt    
│   Makefile  
  ``` 


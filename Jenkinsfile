library "alauda-cicd"
def language = "golang"
AlaudaPipeline {
    config = [
        agent: 'golang-1.13',
        folder: '.',
        scm: [
            credentials: 'acp-acp-gitlab'
        ],
        docker: [
            repository: "acp/topolvm-operator",
            credentials: "alaudak8s",
            context: ".",
            dockerfile: "Dockerfile",
            armBuild: true,
        ],
        sonar: [
            binding: "sonarqube",
            enabled: true,
        ],
        sec: [
            enabled: true,
            block: false,
            lang: 'go',
            scanMod: 1,
            customOpts: ''
        ],
	notification: [
	    name: "default"
	],
    ]
    env = [
        GO111MODULE: "on",
    ]
    steps = [
    ]

}

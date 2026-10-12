# Imagem do job de deploy (deploy_staging): o que o before_script do template
# instalava a cada execução — cliente SSH e Ansible — já vem pronto. O
# before_script continua rodando igual, só que os passos de instalação viram
# "já instalado".
#
# Mudou este arquivo? Suba DEPLOY_IMAGE_TAG no .gitlab-ci.yml: é o nome da
# imagem, e o job imagem:deploy só constrói a tag que ainda não existe.
FROM python:3.13-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends openssh-client \
    && rm -rf /var/lib/apt/lists/*

# A mesma versão que o `pip install ansible-core` do template instalava
# quando esta imagem foi criada.
RUN pip install --no-cache-dir ansible-core==2.21.5

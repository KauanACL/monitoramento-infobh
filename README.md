# Monitoramento InfoBH

Sistema em Go para monitorar máquinas Windows de clientes usando uma VM Ubuntu como servidor central.

## Componentes

- `cmd/server`: servidor HTTP, API dos agentes, dashboard sem login e banco SQLite.
- `cmd/agent`: agente Windows instalável como serviço.
- SQLite em WAL no caminho `data/monitoramento.db`.
- Retenção padrão de 30 dias para métricas detalhadas, eventos e eventos de dispositivos.

## Rodar localmente

```bash
go run ./cmd/server -addr :8080
```

Acesse:

```text
http://localhost:8080
```

## Build do servidor para Ubuntu

Na VM Ubuntu:

```bash
sudo useradd --system --home /opt/monitoramento-infobh --shell /usr/sbin/nologin monitoramento
sudo mkdir -p /opt/monitoramento-infobh/data
sudo chown -R monitoramento:monitoramento /opt/monitoramento-infobh

go build -o monitor-server ./cmd/server
sudo cp monitor-server /opt/monitoramento-infobh/
sudo cp deploy/monitoramento-infobh.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now monitoramento-infobh
```

Liberar porta:

```bash
sudo ufw allow 8080/tcp
```

Dashboard:

```text
http://IP_DA_VM:8080
```

## Build do agente Windows

No Mac/Linux:

```bash
GOOS=windows GOARCH=amd64 go build -o bin/infobh-agent.exe ./cmd/agent
```

No dashboard, crie um cliente e uma máquina. A tela vai mostrar o token e o comando de instalação.

No Windows, execute como Administrador:

```powershell
.\infobh-agent.exe install -server http://IP_DA_VM:8080 -token TOKEN_GERADO
```

Comandos úteis:

```powershell
.\infobh-agent.exe once -server http://IP_DA_VM:8080 -token TOKEN_GERADO
.\infobh-agent.exe stop
.\infobh-agent.exe start
.\infobh-agent.exe restart
.\infobh-agent.exe uninstall
.\infobh-agent.exe config-path
```

## Coleta

- Heartbeat: a cada 30 segundos.
- Métricas: a cada 60 segundos.
- Dispositivos: a cada 5 minutos.
- Offline: máquina sem heartbeat por mais de 2 minutos.

O agente coleta:

- CPU.
- RAM.
- discos/SSD/HD e armazenamento, incluindo total, usado, livre e percentual usado.
- internet/conectividade.
- dispositivos USB.
- impressoras.

## Segurança

O dashboard foi implementado sem login, conforme definido no plano. Se a porta `8080` estiver aberta na internet, qualquer pessoa com o IP acessa os dados. Os endpoints dos agentes exigem token por máquina.

Para produção, o recomendado é colocar firewall por IP, VPN ou migrar o dashboard para HTTPS com autenticação.

## Variáveis do servidor

```bash
MONITOR_ADDR=:8080
MONITOR_DB_PATH=data/monitoramento.db
MONITOR_RETENTION_DAYS=30
```

Também podem ser usadas como flags:

```bash
./monitor-server -addr :8080 -db data/monitoramento.db -retention-days 30
```

## Deploy no Render.com

O projeto já inclui `render.yaml` para criar um Web Service Go com SQLite em Persistent Disk.

Recomendado para produção:

- Web Service pago (`starter` ou superior).
- Persistent Disk montado em `/data`.
- `MONITOR_DB_PATH=/data/monitoramento.db`.
- `MONITOR_RETENTION_DAYS=30`.

O Render define a porta em `PORT` automaticamente; o servidor usa essa variável quando `MONITOR_ADDR` não está definido.

Passos:

1. Envie este projeto para um repositório GitHub.
2. No Render, clique em **New > Blueprint**.
3. Conecte o repositório.
4. Confirme o serviço `monitoramento-infobh`.
5. Aguarde o deploy.

Depois do deploy, use a URL `https://SEU-SERVICO.onrender.com` no agente:

```powershell
.\infobh-agent.exe install -server https://SEU-SERVICO.onrender.com -token TOKEN_GERADO
```

O deploy no Render também gera o agente Windows 64-bit para download em:

```text
https://SEU-SERVICO.onrender.com/static/downloads/infobh-agent-windows-amd64.exe
```

Observações:

- Sem Persistent Disk, o SQLite perde dados em restart/deploy.
- Plano gratuito não é recomendado para monitoramento porque pode dormir e atrasar heartbeats.
- O dashboard segue sem login; qualquer pessoa com a URL consegue acessar.

## Testes

```bash
go test ./...
```

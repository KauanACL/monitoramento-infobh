# Monitoramento InfoBH

Sistema em Go para monitorar máquinas Windows de clientes usando uma VM Ubuntu como servidor central.

## Componentes

- `cmd/server`: servidor HTTP, API dos agentes, dashboard sem login e banco SQLite.
- `cmd/agent`: agente Windows instalável como serviço.
- SQLite em WAL no caminho `data/monitoramento.db`.
- Retenção padrão de 30 dias para métricas detalhadas, eventos e eventos de dispositivos.
- Alertas globais configuráveis para CPU, RAM e armazenamento.

## Rodar localmente

```bash
go run ./cmd/server -addr :8080
```

Acesse:

```text
http://localhost:8080
```

## Deploy em VM Ubuntu

Compile o servidor Linux na sua máquina local:

```bash
GOOS=linux GOARCH=amd64 go build -o bin/monitor-server-linux-amd64 .
```

Copie para a VM:

```bash
scp bin/monitor-server-linux-amd64 usuario@IP_DA_VM:/tmp/monitor-server
scp deploy/monitoramento-infobh.service usuario@IP_DA_VM:/tmp/monitoramento-infobh.service
```

Na VM Ubuntu:

```bash
sudo useradd --system --home /opt/monitoramento-infobh --shell /usr/sbin/nologin monitoramento
sudo mkdir -p /opt/monitoramento-infobh/data
sudo cp /tmp/monitor-server /opt/monitoramento-infobh/monitor-server
sudo cp /tmp/monitoramento-infobh.service /etc/systemd/system/
sudo chmod +x /opt/monitoramento-infobh/monitor-server
sudo chown -R monitoramento:monitoramento /opt/monitoramento-infobh
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

Ver logs:

```bash
sudo systemctl status monitoramento-infobh --no-pager
sudo journalctl -u monitoramento-infobh -f
```

## Build do agente Windows

Os agentes baixáveis ficam em `internal/server/web/static/downloads/` e são servidos pelo próprio dashboard.

Para recompilar os agentes:

```bash
GOOS=windows GOARCH=amd64 go build -o internal/server/web/static/downloads/infobh-agent-windows-amd64.exe ./cmd/agent
GOOS=windows GOARCH=386 go build -o internal/server/web/static/downloads/infobh-agent-windows-386.exe ./cmd/agent
```

No dashboard, crie um cliente e uma máquina. A tela vai mostrar o token e o comando de instalação.

No Windows 64-bit, baixe:

```text
http://IP_DA_VM:8080/static/downloads/infobh-agent-windows-amd64.exe
```

Abra o Prompt/PowerShell como Administrador, entre na pasta onde o arquivo foi baixado e rode primeiro um teste:

```powershell
.\infobh-agent-windows-amd64.exe once -server http://IP_DA_VM:8080 -token TOKEN_GERADO
```

Se o teste não retornar erro, instale como serviço:

```powershell
.\infobh-agent-windows-amd64.exe install -server http://IP_DA_VM:8080 -token TOKEN_GERADO
```

Comandos úteis:

```powershell
.\infobh-agent-windows-amd64.exe stop
.\infobh-agent-windows-amd64.exe start
.\infobh-agent-windows-amd64.exe restart
.\infobh-agent-windows-amd64.exe uninstall
.\infobh-agent-windows-amd64.exe config-path
sc query InfoBHMonitorAgent
```

## Coleta

- Heartbeat: a cada 30 segundos.
- Métricas: a cada 60 segundos.
- Dispositivos: a cada 5 minutos.
- Hardware: ao iniciar e depois a cada 6 horas.
- Temperaturas nativas: a cada 60 segundos, quando o Windows expuser sensores.
- Comandos remotos: agente busca comandos a cada 15 segundos.
- Offline: máquina sem heartbeat por mais de 2 minutos.

O agente coleta:

- CPU.
- RAM e slots quando o Windows informar.
- discos/SSD/HD e armazenamento, incluindo total, usado, livre e percentual usado.
- internet/conectividade.
- dispositivos USB.
- impressoras.
- inventário de CPU, placa/sistema e módulos de memória.
- temperaturas nativas via WMI/CIM quando disponíveis.

## Segurança

O dashboard foi implementado sem login, conforme definido no plano. Se a porta `8080` estiver aberta na internet, qualquer pessoa com o IP acessa os dados. Os endpoints dos agentes exigem token por máquina. Ações que alteram estado, como salvar limites de alerta e limpar temporários no Windows, usam o PIN fixo `110680`.

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

## Testes

```bash
go test ./...
```

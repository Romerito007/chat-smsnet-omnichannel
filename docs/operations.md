# Operação & Deploy

Requisitos de ambiente que **quem sobe o chat precisa configurar** na
infraestrutura (além das variáveis do `.env`). São pré-condições operacionais:
o binário sobe sem elas, mas alguns comportamentos ao vivo só funcionam se o
ambiente estiver configurado como descrito aqui.

---

## Redis: keyspace notifications de expiração (`notify-keyspace-events`)

### Por que é necessário

A presença dos agentes ("Online/Offline" no painel de supervisão) usa uma
**chave Redis com TTL** como sinal de vida (`presence:<tenant>:<user>`,
renovada a cada heartbeat do WebSocket). Quando um agente **cai de forma
abrupta** — fecha a aba sem deslogar, perde conexão, mata o navegador — não há
disconnect gracioso: a chave simplesmente **expira** quando o heartbeat para de
renovar.

Para que essa expiração vire um evento `agent.presence_changed` (status
`offline`) **ao vivo** nos painéis, o backend assina os *keyspace expired
events* do Redis (canal `__keyevent@<db>__:expired`) e, ao receber a chave
expirada, publica o offline. Esse subscriber é a **Opção A** da detecção de
expiração.

**Se `notify-keyspace-events` não incluir os eventos de expiração**, o Redis
expira a chave silenciosamente, **sem emitir o evento**. O subscriber nunca é
acordado e o painel **não atualiza** — o agente continua aparecendo "Online"
até alguém dar **refresh** na página.

### Sintoma se esquecer

Volta o **fantasma "Online"**: agentes que caíram abruptamente (queda de
conexão / aba fechada sem logout) continuam listados como "Online" no quadro de
supervisão até um refresh manual. O *fast-path* de disconnect gracioso (quando
o socket fecha de forma limpa) **continua funcionando** — o que deixa de
atualizar ao vivo são exatamente as **quedas não-graciosas**.

### Como habilitar

O backend já tenta habilitar isso sozinho no start (best-effort:
`CONFIG SET notify-keyspace-events Ex`, com warning no log se o Redis recusar —
caso comum em Redis **gerenciado**, que bloqueia `CONFIG SET`). Mesmo assim,
**garanta** que está ativo, nos dois modos:

**Runtime (sem reiniciar o Redis) — vale até o próximo restart do Redis:**

_Redis no host, sem senha:_

```bash
redis-cli CONFIG SET notify-keyspace-events Ex
```

_Redis com senha (`REDIS_PASSWORD`):_ adicione `-a <REDIS_PASSWORD>` (sem o `-a`
o Redis responde `NOAUTH Authentication required`).

```bash
redis-cli -a <REDIS_PASSWORD> CONFIG SET notify-keyspace-events Ex
```

_Redis em container Docker (com senha) — execute dentro do container:_

```bash
docker exec -it <container> redis-cli -a <REDIS_PASSWORD> CONFIG GET notify-keyspace-events
docker exec -it <container> redis-cli -a <REDIS_PASSWORD> CONFIG SET notify-keyspace-events Ex
```

**Persistente (sobrevive a restart):**

_Redis no host — no `redis.conf`:_

```conf
notify-keyspace-events Ex
```

…e reinicie o serviço (`systemctl restart redis` ou equivalente).

_Docker Compose (com senha) — no `command` do serviço Redis:_

```yaml
services:
  redis:
    image: redis:7
    command: redis-server --requirepass <REDIS_PASSWORD> --notify-keyspace-events Ex
```

> Significado dos flags: **`E`** = *keyevent* notifications (canal
> `__keyevent@<db>__:<evento>`), **`x`** = eventos de **expiração**. O backend
> precisa exatamente de `E` + `x`.
>
> Em todos os exemplos abaixo, se o seu Redis tem senha, acrescente
> `-a <REDIS_PASSWORD>` ao `redis-cli`; se roda em container, prefixe com
> `docker exec -it <container>`.

### ⚠️ Cuidado: não sobrescreva uma config existente

`notify-keyspace-events` é **um conjunto de flags numa única string**. Um
`CONFIG SET` **substitui** o valor inteiro — ele não soma. Se o Redis já usa
keyspace notifications para outra finalidade (ex.: `KEA`, `Elx`, `KEx`…),
**setar só `Ex` apaga os flags anteriores**.

**Sempre verifique o valor atual antes de setar:**

```bash
redis-cli CONFIG GET notify-keyspace-events
# ex.: 1) "notify-keyspace-events"
#      2) "KEA"
```

E então **adicione** `E` e `x` aos flags existentes, sem remover os demais.
Exemplos:

| Valor atual | O que fazer | Valor final |
|---|---|---|
| vazio (`""`) | setar `Ex` | `Ex` |
| `KEA` | já cobre tudo (`A` = todos os eventos, inclui expirados) — **nada a fazer** | `KEA` |
| `Kx` | falta `E` | `KEx` |
| `El` | falta `x` | `Elx` |
| `KE` | falta `x` | `KEx` |

> Regra prática: o valor final precisa conter **`E`** *e* **`x`** (ou então
> **`A`** junto de `E`, já que `A` agrega todos os tipos de evento). Mantenha
> quaisquer outros flags (`K`, `g`, `l`, `s`, `h`, `z`, …) que já estavam lá.

### ⚠️ O evento de expiração é POR DATABASE — use o `REDIS_DB` certo

Os *keyspace expired events* são **escopados por database lógico** do Redis: uma
chave que expira no DB **N** emite no canal `__keyevent@N__:expired`. Se o chat
roda em `REDIS_DB=4`, as chaves de presença expiram no **DB 4** e o evento sai em
`__keyevent@4__:expired` — **não** em `@0`.

O subscriber monta o canal **dinamicamente a partir do `REDIS_DB` configurado**
(`__keyevent@{REDIS_DB}__:expired`) e usa o **mesmo client Redis** do resto do
app, então grava a presença e escuta a expiração **no mesmo database**. Você não
precisa fazer nada além de manter o `REDIS_DB` consistente — só **não** aponte o
subscriber e o app para DBs diferentes.

`notify-keyspace-events` é **global** (não por DB): habilitar uma vez cobre todos
os databases.

### Como verificar que funcionou

> Os comandos abaixo são na variante host-sem-senha. Com senha, acrescente
> `-a <REDIS_PASSWORD>`; em Docker, prefixe com `docker exec -it <container>`.

1. **Confirme a config** ativa no Redis:

   ```bash
   redis-cli CONFIG GET notify-keyspace-events   # precisa conter E e x (ou A+E)
   ```

2. **Observe o canal direto** (no DB que o chat usa, ex. 4). Em um terminal:

   ```bash
   redis-cli PSUBSCRIBE '__keyevent@4__:expired'
   ```

   Em outro, force uma expiração rápida e veja se chega no primeiro:

   ```bash
   redis-cli -n 4 SET presence:teste:teste x EX 1
   # ~1s depois, o PSUBSCRIBE imprime:
   #   pmessage __keyevent@4__:expired __keyevent@4__:expired presence:teste:teste
   ```

   Se **nada** aparece, os expired events não estão ativos (revise os flags / DB).

   _Mesma verificação no seu ambiente (Docker + senha, DB 4):_

   ```bash
   # terminal 1 — escuta o canal de expiração do DB 4
   docker exec -it <container> redis-cli -a <REDIS_PASSWORD> --csv PSUBSCRIBE '__keyevent@4__:expired'

   # terminal 2 — força uma chave a expirar em 1s no DB 4
   docker exec -it <container> redis-cli -a <REDIS_PASSWORD> -n 4 SET presence:teste:teste x EX 1
   ```

3. **No log do backend** (papel `ws` ou `all`): no start sai
   `presence expiry watcher started channel=__keyevent@4__:expired`. Ao derrubar
   um agente de verdade (feche a aba e espere o TTL de ~60s), o offline é
   publicado no tópico `presence`; se algo falhar, sai um warn
   `presence expiry: vanish failed …`.

### Resumo

| | |
|---|---|
| **Flag necessária** | `notify-keyspace-events` contendo `E` + `x` (ou `A`+`E`) |
| **Escopo** | global (uma vez cobre todos os DBs) |
| **Canal escutado** | `__keyevent@{REDIS_DB}__:expired` (dinâmico pelo env) |
| **Sem isso** | fantasma "Online" em quedas abruptas até o refresh |
| **Disconnect gracioso** | independe disto (fast-path próprio, ~5s) |

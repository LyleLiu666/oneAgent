# n8n Top Issues (Open & High Reactions)

Fetched at: Fri Jan 30 22:32:26 CST 2026
Repo: https://github.com/n8n-io/n8n

## ["IDs of workflows that can call this one" Field Rejects Workflow IDs Containing Hyphens](https://github.com/n8n-io/n8n/issues/25092) (#25092)
**Created**: 2026-01-30T13:31:08Z

### Bug Description

The "IDs of workflows that can call this one" field in workflow settings does not accept workflow IDs that contain hyphens (-). The field's placeholder shows e.g. 14, 18 suggesting it only expects numeric IDs, but n8n now generates nanoid-style workflow IDs that can include hyphens.

<img width="242" height="38" alt="Image" src="https://github.com/user-attachments/assets/de2ea0f7-938a-46f5-98ec-6ee86ff9df8a" />

<img width="1129" height="714" alt="Image" src="https://github.com/user-attachments/assets/0d94a720-c077-4f2d-ab25-015ebbd489ac" />

### To Reproduce

Have a workflow with an ID containing a hyphen (e.g., aM38pcOitI4itMwZdB-kJ)
Open another workflow's settings
Set "This workflow can be called by" to "Selected workflows"
Attempt to enter aM38pcOitI4itMwZdB-kJ in the "IDs of workflows that can call this one" field
Observe that the input is rejected or cannot be saved


### Expected behavior

The field should accept valid workflow IDs in any format n8n generates, including IDs with hyphens like aM38pcOitI4itMwZdB-kJ.

### Debug Info

# Debug info

## core

- n8nVersion: 2.4.8
- platform: docker (self-hosted)
- nodeJsVersion: 22.21.1
- nodeEnv: production
- database: postgres
- executionMode: scaling (multi-main)
- concurrency: -1
- license: enterprise (production)
- consumerId: 507fd647-278d-418c-8e5b-e4fbfecd6aaf

## storage

- success: all
- error: all
- progress: false
- manual: true
- binaryMode: memory

## pruning

- enabled: true
- maxAge: 336 hours
- maxCount: 10000 executions

## client

- userAgent: mozilla/5.0 (macintosh; intel mac os x 10_15_7) applewebkit/537.36 (khtml, like gecko) chrome/144.0.0.0 safari/537.36
- isTouchDevice: false

Generated at: 2026-01-30T13:26:55.286Z

### Operating System

Azure AKS - Ubuntu Nodes - Multi Main - Queue Mode

### n8n Version

2.4.8

### Node.js Version

Docker Image (Node.js >= 22.16)

### Database

PostgreSQL

### Execution mode

queue

### Hosting

self hosted

---

## [The service is receiving too many requests from you Too many requests, possible issue on schedul trigger?](https://github.com/n8n-io/n8n/issues/25091) (#25091)
**Created**: 2026-01-30T13:22:28Z

### Bug Description

i have issue with my self-hosted service. is very strange; if i start the workflow manually, all works, if I start the workflow via scheduling, the issue appears. 

```
The service is receiving too many requests from you
Too many requests
```

im try to call my browserless service self hosted on another server. 

i have try to set the batch options but not solve and again appear just when the workflow start with scheduling.

maybe some issue on schedule trigger? 

### To Reproduce

```
{
  "nodes": [
    {
      "parameters": {
        "method": "POST",
        "url": "https://browserless-aifbofficial.app.rewamp.it/download",
        "sendQuery": true,
        "queryParameters": {
          "parameters": [
            {
              "name": "token",
              "value": "xxxxx"
            },
            {
              "name": "timeout",
              "value": "300000"
            }
          ]
        },
        "sendHeaders": true,
        "headerParameters": {
          "parameters": [
            {
              "name": "Cache-Control",
              "value": "no-cache"
            }
          ]
        },
        "sendBody": true,
        "contentType": "raw",
        "rawContentType": "application/javascript",
        "body": "=export default async function ({ page, browser }) {\n  try {\n    // Funzione helper per il timeout\n    const wait = (ms) => new Promise((resolve) => setTimeout(resolve, ms));\n\n    console.log(\"Inizia navigazione...\");\n    // Naviga alla pagina di login\n    await page.goto(\n      \"https://srv1.intiway.it/IwApiS/actions.amministrazione.ActionGestionePassword.do\",\n      { waitUntil: 'networkidle0', timeout: 60000 }\n    );\n\n    console.log(\"Effettua il login...\");\n    // Effettua il login\n    await page.type('input[name=\"cod_user\"]', \"ADMIN\");\n    await page.type('input[name=\"password\"]', \"acn4rny.WDW.dvh8cux\");\n    \n    // Clicca il pulsante login e ASPETTA che la navigazione sia completata\n    await Promise.all([\n      page.waitForNavigation({ waitUntil: 'networkidle0', timeout: 30000 }),\n      page.click('input[type=\"image\"][alt=\"invia\"]')\n    ]);\n\n    console.log(\"Login completato, navigo alla pagina estrazioni...\");\n    // Naviga alla pagina desiderata\n    await page.goto(\n      \"https://srv1.intiway.it/IwApiS/it.mediasoft.estrazioni.actions.ActionInitEstrazioni.do?TAB=ANDVEND&TITOLO=Andamento%20Venduto\",\n      { waitUntil: 'networkidle0', timeout: 60000 }\n    );\n\n    console.log(\"Attendo che i frame si carichino...\");\n    // Attendi che i frame si carichino\n    await wait(3000);\n\n    // Trova il frame \"HEAD\"\n    const findHeadFrame = () => {\n      return page\n        .frames()\n        .find(\n          (frame) =>\n            frame.name() === \"HEAD\" || frame.url().includes(\"HeadEstrAut.jsp\")\n        );\n    };\n\n    let headFrame = findHeadFrame();\n    if (!headFrame) {\n      throw new Error('Frame \"HEAD\" non trovato');\n    }\n\n    console.log(\"Frame HEAD trovato, compilo i campi data...\");\n    // Interagisci con i campi data\n    await headFrame.evaluate(() => {\n      document.querySelector('input[name=\"ddData\"]').value = \"{{ $json.date.toDateTime().format('dd') }}\";\n      document.querySelector('input[name=\"mmData\"]').value = \"{{ $json.date.toDateTime().format('LL') }}\";\n      document.querySelector('input[name=\"yyData\"]').value = \"{{ $json.date.toDateTime().format('yyyy') }}\";\n    });\n\n    console.log(\"Clicco Elabora...\");\n    // Clicca Elabora e attendi che finisca\n    await headFrame.click('input[type=\"image\"][src*=\"Elabora.gif\"]');\n    console.log(\"Elaborazione avviata...\");\n\n    // Attendi l'elaborazione (aumentato a 60 secondi)\n    await wait(90000);\n\n    console.log(\"Ricerco il frame dopo elaborazione...\");\n    // Riacquisisci il frame dopo l'elaborazione\n    headFrame = findHeadFrame();\n    if (!headFrame) {\n      throw new Error('Frame \"HEAD\" non trovato dopo elaborazione');\n    }\n\n    console.log(\"Attendo che il pulsante download sia disponibile...\");\n    // Attendi che il pulsante download sia disponibile\n    await headFrame.waitForSelector('#divDownload img[alt=\"Download\"]', {\n      visible: true,\n      timeout: 10000,\n    });\n\n    console.log(\"Clicco il pulsante download...\");\n    // Clicca il pulsante download - Browserless gestirà automaticamente il download\n    await headFrame.click('#divDownload img[alt=\"Download\"]');\n    console.log(\"Pulsante download cliccato\");\n\n    // Attendi che il download si completi (aumentato a 5 secondi)\n    console.log(\"Attendo completamento download...\");\n    await wait(5000);\n    \n    console.log(\"Download completato!\");\n  } catch (error) {\n    console.error(\"Errore durante l'esecuzione:\", error);\n    throw error;\n  } finally {\n    if (browser) {\n      await browser.close();\n      console.log(\"Browser chiuso\");\n    }\n  }\n}",
        "options": {
          "timeout": 600000
        }
      },
      "type": "n8n-nodes-base.httpRequest",
      "typeVersion": 4.2,
      "position": [
        448,
        -48
      ],
      "id": "c35b45fc-7bba-4a13-a0ae-f08c019fd561",
      "name": "Scrape anven final"
    },
    {
      "parameters": {},
      "type": "n8n-nodes-base.manualTrigger",
      "typeVersion": 1,
      "position": [
        -224,
        -48
      ],
      "id": "96688363-0538-46f4-9188-90f7fea068a5",
      "name": "When clicking ‘Execute workflow’"
    },
    {
      "parameters": {
        "assignments": {
          "assignments": [
            {
              "id": "a5bef8b3-c4c6-4cb7-93e8-2342a0121421",
              "name": "date",
              "value": "={{$now.format('yyyy-MM-dd')}}",
              "type": "string"
            }
          ]
        },
        "options": {}
      },
      "type": "n8n-nodes-base.set",
      "typeVersion": 3.4,
      "position": [
        0,
        -144
      ],
      "id": "dd39dfe4-0218-433c-a96a-a6696ab76a03",
      "name": "Settings"
    },
    {
      "parameters": {
        "method": "POST",
        "url": "https://browserless-aifbofficial.app.rewamp.it/download",
        "sendQuery": true,
        "queryParameters": {
          "parameters": [
            {
              "name": "token",
              "value": "xxxxxxx"
            },
            {
              "name": "timeout",
              "value": "300000"
            }
          ]
        },
        "sendHeaders": true,
        "headerParameters": {
          "parameters": [
            {
              "name": "Cache-Control",
              "value": "no-cache"
            }
          ]
        },
        "sendBody": true,
        "contentType": "raw",
        "rawContentType": "application/javascript",
        "body": "=export default async function ({ page, browser }) {\n  try {\n    // Funzione helper per il timeout\n    const wait = (ms) => new Promise((resolve) => setTimeout(resolve, ms));\n\n    console.log(\"Inizia navigazione...\");\n    // Naviga alla pagina di login\n    await page.goto(\n      \"https://srv1.intiway.it/IwApiS/actions.amministrazione.ActionGestionePassword.do\",\n      { waitUntil: 'networkidle0', timeout: 60000 }\n    );\n\n    console.log(\"Effettua il login...\");\n    // Effettua il login\n    await page.type('input[name=\"cod_user\"]', \"ADMIN\");\n    await page.type('input[name=\"password\"]', \"acn4rny.WDW.dvh8cux\");\n    \n    // Clicca il pulsante login e ASPETTA che la navigazione sia completata\n    await Promise.all([\n      page.waitForNavigation({ waitUntil: 'networkidle0', timeout: 30000 }),\n      page.click('input[type=\"image\"][alt=\"invia\"]')\n    ]);\n\n    console.log(\"Login completato, navigo alla pagina estrazioni...\");\n    // Naviga alla pagina desiderata\n    await page.goto(\n      \"https://srv1.intiway.it/IwApiS/it.mediasoft.estrazioni.actions.ActionInitEstrazioni.do?TAB=ANDVENDDIV&TITOLO=Andamento%20Venduto%20per%20Divisione\",\n      { waitUntil: 'networkidle0', timeout: 60000 }\n    );\n\n    console.log(\"Attendo che i frame si carichino...\");\n    // Attendi che i frame si carichino\n    await wait(3000);\n\n    // Trova il frame \"HEAD\"\n    const findHeadFrame = () => {\n      return page\n        .frames()\n        .find(\n          (frame) =>\n            frame.name() === \"HEAD\" || frame.url().includes(\"HeadEstrAut.jsp\")\n        );\n    };\n\n    let headFrame = findHeadFrame();\n    if (!headFrame) {\n      throw new Error('Frame \"HEAD\" non trovato');\n    }\n\n    console.log(\"Frame HEAD trovato, compilo i campi data...\");\n    // Interagisci con i campi data\n    await headFrame.evaluate(() => {\n      document.querySelector('input[name=\"ddData\"]').value = \"{{ $json.date.toDateTime().format('dd') }}\";\n      document.querySelector('input[name=\"mmData\"]').value = \"{{ $json.date.toDateTime().format('LL') }}\";\n      document.querySelector('input[name=\"yyData\"]').value = \"{{ $json.date.toDateTime().format('yyyy') }}\";\n    });\n\n    console.log(\"Clicco Elabora...\");\n    // Clicca Elabora e attendi che finisca\n    await headFrame.click('input[type=\"image\"][src*=\"Elabora.gif\"]');\n    console.log(\"Elaborazione avviata...\");\n\n    // Attendi l'elaborazione (aumentato a 60 secondi)\n    await wait(90000);\n\n    console.log(\"Ricerco il frame dopo elaborazione...\");\n    // Riacquisisci il frame dopo l'elaborazione\n    headFrame = findHeadFrame();\n    if (!headFrame) {\n      throw new Error('Frame \"HEAD\" non trovato dopo elaborazione');\n    }\n\n    console.log(\"Attendo che il pulsante download sia disponibile...\");\n    // Attendi che il pulsante download sia disponibile\n    await headFrame.waitForSelector('#divDownload img[alt=\"Download\"]', {\n      visible: true,\n      timeout: 10000,\n    });\n\n    console.log(\"Clicco il pulsante download...\");\n    // Clicca il pulsante download - Browserless gestirà automaticamente il download\n    await headFrame.click('#divDownload img[alt=\"Download\"]');\n    console.log(\"Pulsante download cliccato\");\n\n    // Attendi che il download si completi (aumentato a 5 secondi)\n    console.log(\"Attendo completamento download...\");\n    await wait(5000);\n    \n    console.log(\"Download completato!\");\n  } catch (error) {\n    console.error(\"Errore durante l'esecuzione:\", error);\n    throw error;\n  } finally {\n    if (browser) {\n      await browser.close();\n      console.log(\"Browser chiuso\");\n    }\n  }\n}",
        "options": {
          "timeout": 600000
        }
      },
      "type": "n8n-nodes-base.httpRequest",
      "typeVersion": 4.2,
      "position": [
        448,
        -240
      ],
      "id": "360318b4-114f-45fe-8671-bad7ac9f1f66",
      "name": "Scrape anven divisione"
    },
    {
      "parameters": {
        "method": "POST",
        "url": "https://devdash.apisjob.it/api/v1/anven/import/divisione",
        "authentication": "genericCredentialType",
        "genericAuthType": "httpHeaderAuth",
        "sendHeaders": true,
        "headerParameters": {
          "parameters": [
            {
              "name": "Content-Type",
              "value": "application/json"
            }
          ]
        },
        "sendBody": true,
        "bodyParameters": {
          "parameters": [
            {
              "name": "data",
              "value": "={{ $json.data }}"
            },
            {
              "name": "data_rif",
              "value": "={{ $json.data_rif }}"
            }
          ]
        },
        "options": {}
      },
      "type": "n8n-nodes-base.httpRequest",
      "typeVersion": 4.3,
      "position": [
        896,
        -240
      ],
      "id": "ff94ec6a-f059-42ef-a5f2-1a6a1b0ec622",
      "name": "HTTP Request",
      "credentials": {
        "httpHeaderAuth": {
          "id": "w21M6EgvqFi5fniJ",
          "name": "APISJOB DASHBOARD REST API KEY"
        }
      }
    },
    {
      "parameters": {
        "assignments": {
          "assignments": [
            {
              "id": "ed6c21c8-ef75-4e80-abc6-01f012029d14",
              "name": "data",
              "value": "={{ $json.data }}",
              "type": "string"
            },
            {
              "id": "1dc5373e-3b36-4b05-98af-0d642935f1b9",
              "name": "data_rif",
              "value": "={{ $('Settings').item.json.date }}",
              "type": "string"
            }
          ]
        },
        "options": {}
      },
      "type": "n8n-nodes-base.set",
      "typeVersion": 3.4,
      "position": [
        672,
        -240
      ],
      "id": "20db1923-33b8-41df-9d75-ea491a3fbcbd",
      "name": "Edit Fields"
    },
    {
      "parameters": {
        "assignments": {
          "assignments": [
            {
              "id": "ed6c21c8-ef75-4e80-abc6-01f012029d14",
              "name": "data",
              "value": "={{ $json.data }}",
              "type": "string"
            },
            {
              "id": "1dc5373e-3b36-4b05-98af-0d642935f1b9",
              "name": "data_rif",
              "value": "={{ $('Settings').item.json.date }}",
              "type": "string"
            }
          ]
        },
        "options": {}
      },
      "type": "n8n-nodes-base.set",
      "typeVersion": 3.4,
      "position": [
        672,
        -48
      ],
      "id": "14ff1b82-d310-4391-bf41-9947c22e6fd5",
      "name": "Edit Fields1"
    },
    {
      "parameters": {
        "method": "POST",
        "url": "https://devdash.apisjob.it/api/v1/anven/import",
        "authentication": "genericCredentialType",
        "genericAuthType": "httpHeaderAuth",
        "sendHeaders": true,
        "headerParameters": {
          "parameters": [
            {
              "name": "Content-Type",
              "value": "application/json"
            }
          ]
        },
        "sendBody": true,
        "bodyParameters": {
          "parameters": [
            {
              "name": "data",
              "value": "={{ $json.data }}"
            },
            {
              "name": "data_rif",
              "value": "={{ $('Settings').item.json.date }}"
            }
          ]
        },
        "options": {}
      },
      "type": "n8n-nodes-base.httpRequest",
      "typeVersion": 4.3,
      "position": [
        896,
        -48
      ],
      "id": "82aab7a9-d2ff-47e8-8be0-8568609f3e6a",
      "name": "HTTP Request2",
      "credentials": {
        "httpHeaderAuth": {
          "id": "w21M6EgvqFi5fniJ",
          "name": "APISJOB DASHBOARD REST API KEY"
        }
      }
    },
    {
      "parameters": {
        "mode": "chooseBranch"
      },
      "type": "n8n-nodes-base.merge",
      "typeVersion": 3.2,
      "position": [
        1120,
        -144
      ],
      "id": "4b912b19-fed6-4654-90b2-5f05108d8688",
      "name": "Merge"
    },
    {
      "parameters": {
        "conditions": {
          "options": {
            "caseSensitive": true,
            "leftValue": "",
            "typeValidation": "strict",
            "version": 3
          },
          "conditions": [
            {
              "id": "c1345a82-ddc1-4c73-9071-29e1a3838044",
              "leftValue": "={{ $json.success }}",
              "rightValue": true,
              "operator": {
                "type": "boolean",
                "operation": "equals"
              }
            }
          ],
          "combinator": "and"
        },
        "options": {}
      },
      "type": "n8n-nodes-base.if",
      "typeVersion": 2.3,
      "position": [
        1344,
        -144
      ],
      "id": "d9a4d07e-831e-4e1d-9dda-dddd47552bec",
      "name": "If"
    },
    {
      "parameters": {
        "rule": {
          "interval": [
            {
              "triggerAtHour": 8
            }
          ]
        }
      },
      "type": "n8n-nodes-base.scheduleTrigger",
      "typeVersion": 1.3,
      "position": [
        -224,
        -240
      ],
      "id": "1e1c3aef-1206-4725-90a1-ed369b6e3ef8",
      "name": "Schedule Trigger"
    },
    {
      "parameters": {
        "method": "POST",
        "url": "https://devdash.apisjob.it/api/v1/anven/send-email",
        "authentication": "genericCredentialType",
        "genericAuthType": "httpHeaderAuth",
        "sendHeaders": true,
        "headerParameters": {
          "parameters": [
            {
              "name": "Content-Type",
              "value": "application/json"
            }
          ]
        },
        "sendBody": true,
        "bodyParameters": {
          "parameters": [
            {
              "name": "data_rif",
              "value": "={{ $('Settings').item.json.date }}"
            }
          ]
        },
        "options": {}
      },
      "type": "n8n-nodes-base.httpRequest",
      "typeVersion": 4.3,
      "position": [
        1664,
        -144
      ],
      "id": "83efad39-a029-4420-b3df-fb7336da2091",
      "name": "HTTP sendemail",
      "credentials": {
        "httpHeaderAuth": {
          "id": "w21M6EgvqFi5fniJ",
          "name": "APISJOB DASHBOARD REST API KEY"
        }
      }
    }
  ],
  "connections": {
    "Scrape anven final": {
      "main": [
        [
          {
            "node": "Edit Fields1",
            "type": "main",
            "index": 0
          }
        ]
      ]
    },
    "When clicking ‘Execute workflow’": {
      "main": [
        [
          {
            "node": "Settings",
            "type": "main",
            "index": 0
          }
        ]
      ]
    },
    "Settings": {
      "main": [
        [
          {
            "node": "Scrape anven divisione",
            "type": "main",
            "index": 0
          },
          {
            "node": "Scrape anven final",
            "type": "main",
            "index": 0
          }
        ]
      ]
    },
    "Scrape anven divisione": {
      "main": [
        [
          {
            "node": "Edit Fields",
            "type": "main",
            "index": 0
          }
        ]
      ]
    },
    "HTTP Request": {
      "main": [
        [
          {
            "node": "Merge",
            "type": "main",
            "index": 1
          }
        ]
      ]
    },
    "Edit Fields": {
      "main": [
        [
          {
            "node": "HTTP Request",
            "type": "main",
            "index": 0
          }
        ]
      ]
    },
    "Edit Fields1": {
      "main": [
        [
          {
            "node": "HTTP Request2",
            "type": "main",
            "index": 0
          }
        ]
      ]
    },
    "HTTP Request2": {
      "main": [
        [
          {
            "node": "Merge",
            "type": "main",
            "index": 0
          }
        ]
      ]
    },
    "Merge": {
      "main": [
        [
          {
            "node": "If",
            "type": "main",
            "index": 0
          }
        ]
      ]
    },
    "If": {
      "main": [
        [
          {
            "node": "HTTP sendemail",
            "type": "main",
            "index": 0
          }
        ]
      ]
    },
    "Schedule Trigger": {
      "main": [
        [
          {
            "node": "Settings",
            "type": "main",
            "index": 0
          }
        ]
      ]
    }
  },
  "pinData": {},
  "meta": {
    "instanceId": "e47f76167b03bf695ef65cea5066c197f3186b659a7a9fac54274e58e53b0df1"
  }
}
```

### Expected behavior

im expecting all work without this issue

### Debug Info

# Debug info

## core

- n8nVersion: 2.4.6
- platform: docker (self-hosted)
- nodeJsVersion: 22.21.1
- nodeEnv: production
- database: postgres
- executionMode: regular
- concurrency: -1
- license: enterprise (production)
- consumerId: 4dc742ae-b057-46cd-bb8e-84dc8501c943

## storage

- success: all
- error: all
- progress: false
- manual: true
- binaryMode: filesystem

## pruning

- enabled: true
- maxAge: 336 hours
- maxCount: 10000 executions

## client

- userAgent: mozilla/5.0 (windows nt 10.0; win64; x64) applewebkit/537.36 (khtml, like gecko) chrome/144.0.0.0 safari/537.36
- isTouchDevice: false

Generated at: 2026-01-30T13:20:05.597Z

### Operating System

Docker with linux

### n8n Version

2.4.6

### Node.js Version

22.21.1

### Database

PostgreSQL

### Execution mode

main (default)

### Hosting

self hosted

---

## [Data Table Relation gets lost, when flow is copied to a project and back to the original location (project or root)](https://github.com/n8n-io/n8n/issues/25083) (#25083)
**Created**: 2026-01-30T12:31:16Z


<!-- Please follow the template below. Skip the questions that are not relevant to you. -->

## Describe the problem/error/question
When moving a flow, that has existing relations to a data table, to a different project, where the data table is not located and therefore not reachable, the flow throws an error (expected). Once you move that flow back to the original project or root, that error persists. It seems like the relation to the data table does not recover.

## What is the error message (if any)?
Could not find the data table: XYZ

## Please share your workflow/screenshots/recording

```
(Select the nodes on your canvas and use the keyboard shortcuts CMD+C/CTRL+C and CMD+V/CTRL+V to copy and paste the workflow.)
⚠️ WARNING ⚠️ If you have sensitive data in your workflow (like API keys), please remove it before sharing.
```


## Share the output returned by the last node
<!-- If you need help with data transformations, please also share your expected output. -->


## Debug info
The only possible way to get that flow running again, is creating a complete new one, copy all the nodes and publish, while unpublishing the old flow. Then the behaviour of the data table nodes recover to normal and the relation works again.

### core

- n8nVersion: 2.4.6
- platform: docker (cloud)
- nodeJsVersion: 22.21.1
- nodeEnv: production
- database: sqlite
- executionMode: regular
- concurrency: 20
- license: enterprise (sandbox)

### storage

- success: all
- error: all
- progress: false
- manual: true
- binaryMode: filesystem

### pruning

- enabled: true
- maxAge: 720 hours
- maxCount: 25000 executions

### client

- userAgent: mozilla/5.0 (macintosh; intel mac os x 10_15_7) applewebkit/537.36 (khtml, like gecko) chrome/144.0.0.0 safari/537.36
- isTouchDevice: false

Generated at: 2026-01-30T12:16:06.397Z}

---

## [Connection Lost error for multiple days](https://github.com/n8n-io/n8n/issues/25082) (#25082)
**Created**: 2026-01-30T12:15:19Z


<!-- Please follow the template below. Skip the questions that are not relevant to you. -->

## Describe the problem/error/question
-I've had the same connection lost error occur on my n8n for multiple days on end. I've tried to fix it using Claude ai to guide me in fixing the issue inside the terminal and it led to a rabbit  hole of solutions to which none of them worked.


## What is the error message (if any)?
-Problem running the workflow, lost connection to the server

## Please share your workflow/screenshots/recording

<img width="1366" height="768" alt="Image" src="https://github.com/user-attachments/assets/b8a18b05-b141-463e-8656-59cbe7291350" />

```
(Select the nodes on your canvas and use the keyboard shortcuts CMD+C/CTRL+C and CMD+V/CTRL+V to copy and paste the workflow.)
⚠️ WARNING ⚠️ If you have sensitive data in your workflow (like API keys), please remove it before sharing.
```


## Share the output returned by the last node
<!-- If you need help with data transformations, please also share your expected output. -->


## Debug info

### core

- n8nVersion: 2.4.5
- platform: docker (self-hosted)
- nodeJsVersion: 22.21.1
- nodeEnv: production
- database: sqlite
- executionMode: regular
- concurrency: -1
- license: enterprise (production)

### storage

- success: all
- error: all
- progress: false
- manual: true
- binaryMode: filesystem

### pruning

- enabled: true
- maxAge: 168 hours
- maxCount: 10000 executions

### client

- userAgent: mozilla/5.0 (windows nt 10.0; win64; x64) applewebkit/537.36 (khtml, like gecko) chrome/122.0.0.0 safari/537.36
- isTouchDevice: false

### security

- secureCookie: false

Generated at: 2026-01-30T12:04:25.068Z}

---

## [[n8n Cloud] Webhook executions stuck in "Queued" status - never start processing](https://github.com/n8n-io/n8n/issues/25080) (#25080)
**Created**: 2026-01-30T11:05:20Z

### Bug Description

All webhook-triggered workflow executions are stuck in "Queued" / "Starting soon" status and never begin processing. This has been happening for over an hour on my n8n Cloud instance.

- Instance: parag16.app.n8n.cloud
- Affected: ALL webhook-triggered workflows
- "Execute Workflow" (integrated mode) works perfectly fine
- Only webhook mode is broken

Multiple execution IDs stuck: 12208, 12209, 12210, etc.
All show "Queued" → "Starting soon" but never actually start.

### To Reproduce

1. Go to any active workflow with a webhook trigger
2. Call the webhook endpoint from browser/Postman/app
3. Check Executions tab - execution shows "Queued" then "Starting soon"
4. Wait indefinitely - execution never starts processing
5. Stopping and retrying doesn't help - new webhooks also get stuck

### Expected behavior

Webhook executions should start processing within seconds, not get stuck in queue indefinitely.

### Debug Info

Instance URL: parag16.app.n8n.cloud
Plan: n8n Cloud (paid)

Symptoms:
- Webhook mode executions: STUCK (status "new" / "Queued" / "Starting soon")
- Integrated mode executions: WORKING (Execute Workflow calls succeed)
- Issue started: ~Jan 30, 2026 around 10:00 UTC
- Stuck execution IDs: 12208, 12209, 12210+

API confirms webhooks never start:
- status: "new"
- startedAt: null
- mode: "webhook"

Workarounds attempted (all failed):
- Deleting stuck executions via API
- Deactivating/reactivating workflows
- Stopping executions from UI
- New webhooks immediately get stuck again

### Operating System

n8n Cloud (managed)

### n8n Version

2.5.0

### Node.js Version

N/A - Cloud

### Database

SQLite (default)

### Execution mode

main (default)

### Hosting

n8n cloud

---


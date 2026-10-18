# Container 0.2.0 source corpus

Normative source: Container/0.2.0, WorkerVersion/0.8.0 and
WorkerDeployment/0.7.0 under spec/forms. These source identities are frozen
in `spec/forms.freeze.json` after answering the five open questions from PR
#17 against the common Host API v2. Freezing does not publish them or establish
Host adoption, support, or runtime qualification. Container/0.1.0 and its
corpus in the parent directory are unchanged.

`contract.mjs` is an optional authoring aid like `../contract.mjs`, which it
reuses for every key 0.1.0 already defines:

- `normalizeContainerSpec(spec)` treats `requiredSensitiveVars` as a set and
  returns it sorted.
- `classifyPrivateInputs(input)` classifies a CREATE or PUT. Its input has
  `operation`, `spec`, `privateInputs`, `previousNames` and `hostPrivateInputs`. `previousNames` is the
  name set last accepted for the Resource. It returns what the new generation
  receives and never returns a value.
- `selectExecutableManifest({sourceDigest, source, selectedConfigPlatform})`
  selects the linux/amd64 manifest from an index pin.
- `selectedImageBytes(manifest, limit)` counts config and layer sizes of the
  selected manifest.

`private-input-cases.json` lists CREATE and PUT cases. `previousNames` is the
name set of the configured secret map, which belongs to the latest accepted
CREATE or PUT generation even when that generation failed; a DELETE
generation never replaces it. `expect.secrets` is `none`,
`set`, `inherit`, `inherit-subset` or `replace`. `valuesChange` is a fixture
aid, not a Form term: it says whether the values delivered to the new
generation can differ from the previous generation's. The Host still compares
replayed values privately and keeps custody under the common API. When
several refusals apply to one request the common API fixes no order, so no
case combines them.

`image-cases.json` lists index selection and selected-byte cases. Its
2,000,000,000-byte inclusive limit is a fixture default for Host admission,
not a Form constant.

`binding-cases.json` adds early-response upload scenarios for
`../binding-suite.mjs`; the runner does not exercise them yet. The test owns
the caller's request ReadableStream, so `uploadSourceCancelled` comes only
from that stream's own `cancel` callback, never from an `AbortController`,
response EOF or adapter bookkeeping. The positive cases also need one backend
event, `uploadPumpClosed`, recorded when the backend's native upload read
ends; `early-response-nonsettling-upload` withholds it and expects the Host to
keep the call unknown for drain and retirement.
`host-cases.json` adds real-backend qualifications to `../host-cases.json`.

`callers.mjs` checks the callers of this URL. WorkerVersion 0.6.0 and 0.7.0
may bind only Container 0.1.0; WorkerVersion 0.8.0 may bind 0.1.0 or 0.2.0 and
seals each target's exact URL. WorkerDeployment 0.7.0 selects WorkerVersion
0.5.0 to 0.8.0 and shares one single-active fence with WorkerDeployment 0.4.0
to 0.6.0. `caller-cases.json` lists its cases.

`successor-deltas.json` records every difference between the frozen
WorkerVersion 0.7.0 and WorkerDeployment 0.6.0 texts and their frozen
successors. The node test checks each predecessor against its freeze hash,
applies the recorded changes in both directions and requires byte equality,
and reads the target lists, version lists, fence and Container limits back from
the texts to compare them with the checkers.

```console
node --test scripts/container-successor.node-test.mjs
```

Passing these checks shows only that the corpus and checker agree. It is not
Host support or runtime qualification.

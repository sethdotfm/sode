# Setup and import comparables

DISCLOSURE: This was written by an LLM.

Research for wayfinder ticket #377 (map #375). Date of reading: 2026-11-14. Every claim cites the vendor's own docs. Where a page could not be read, the Gaps section says so. Nothing here comes from a blog or forum.

## Summary

- Companion and Home Assistant show the two import models Sode needs. Companion offers per-category and per-item choice with two reset modes. Home Assistant offers per-part choice, and its restore sits inside the first-run screen.
- Almost no comparable tool documents what happens to a reference that is missing on the target. Companion tells you to install the needed module versions before importing. Grafana lets you map data sources during import. Sode should design this behavior itself and test it.
- First-run wizards in the self-hosted apps are short (Jellyfin has 6 steps, Home Assistant 3 screens). Access is set by creating one administrator account. None of them asks for an access level at setup. Companion and Ross DashBoard have no wizard at all.

## Bitfocus Companion

**Steps to a working state.** Open the launcher, choose the network interface and port (default 8000), click Launch GUI, then add a connection: find the device, pick the module version, enter its details, save. Each physical device needs its own connection. Sources: [start the server](https://companion.free/user-guide/beta/getting-started/start-the-server), [connections](https://github.com/bitfocus/companion/blob/main/docs/user-guide/3_config/connections.md).

**What it skips.** There is no setup wizard. The start-the-server page describes none, and the admin UI page lists no onboarding ([the user interface](https://companion.free/user-guide/beta/config/the-user-interface)). Buttons, surfaces and triggers are all left to the user.

**Access at setup.** Not chosen at setup. The interface choice in the launcher is the only first-run access decision, because it decides whether other machines can reach the admin page. An Admin UI Password is a later Advanced setting with a session timeout, and the docs say it only stops casual browsers ([settings v5.0](https://companion.free/user-guide/v5.0/config/settings)). The docs say never to expose the interface to the internet ([before you open](https://companion.free/user-guide/v5.0/getting-started/before-you-open)).

**Selective export and import.** Export writes a `.companionconfig` file in Compressed, JSON or YAML form, with notes the importer sees. A single page exports separately from the buttons page ([import/export, beta](https://companion.free/user-guide/beta/config/import-export)). Import has three tabs:

- Full import, where you choose which configuration categories to import. Options the file cannot support are disabled. The docs do not list the categories.
- Buttons, which imports a subset of pages.
- Triggers, which imports a subset of triggers.

Full import has two buttons. "Import, Preserving unselected components" resets only the selected components, repopulates them from the file, and keeps the rest. "Full Reset then Import" resets everything, then imports only the selected categories. The docs recommend exporting the current config first ([import/export, v5.0](https://companion.free/user-guide/v5.0/config/import-export)).

**Missing references.** The Buttons and Triggers tabs offer optional connection remapping. The docs do not say what happens to an unmapped connection. For modules, the docs say to check that the right module versions are installed on the Connections page before importing, and that offline systems may need the offline module bundle ([connections](https://github.com/bitfocus/companion/blob/main/docs/user-guide/3_config/connections.md)). The project tracker has reports of "Missing version information" warnings after import, for example [issue 3253](https://github.com/bitfocus/companion/issues/3253). That is a bug report, not documented behavior.

## Ross DashBoard

**Steps to a working state.** File > New > TCP/IP DashBoard Connect or openGear Device, then IP address, a unique display name, protocol and port, then Finish. The device appears in the tree. Each Ultrix frame, Ultripower and Ultricool needs its own connection, and a device without an IP needs Walkabout to assign one ([To Connect to Ultrix from DashBoard](https://help.rossvideo.com/ultrix-acuity/Tasks/Setup/Ultrix/To-Ultrix-DashBoard.html)). There is no wizard and no account step.

**Access at setup.** DashBoard itself is open. Role-based access is an add-on that needs a Ross Platform Manager server and user accounts. On a device such as a switcher, an operator picks "Authorization Not Required" (RBAC applied but not enforced) or "Authorization Required" (enforced), and can list pre-authorized IP addresses that skip login. The page warns that a wrong mode can lock out an account without the needed role, and that RBAC settings are not saved in switcher sets and are not reset by a factory default ([turn on RBAC](https://help.rossvideo.com/ultra/Tasks/RBAC/RBAC-Control.html), [RBAC overview](https://help.rossvideo.com/graphite/Topics/RBAC/RBAC.html)). This is the closest comparable to a graded access setting: not required, then required, with a trusted-address list.

**Selective export and import.** Not found at the app level. Settings live in a workspace folder, and the docs say to back up by copying it. A device's configuration is saved or restored per device from its right-click menu, and DashBoard saves layouts and custom folder trees separately ([DashBoard user guide](https://documentation.rossvideo.com/files/Manuals%20(ARCHIVED%20DO%20NOT%20USE)/Enterprise%20Control%20Systems/DashBoard/DashBoard_User_Guide.pdf), seen through a search excerpt only, see Gaps). No category or item picker was found.

**Missing references.** Not documented in the pages read.

## Grass Valley router control

**Jupiter and CommandCenter.** The CommandCenter docs say the Jupiter configuration must be done first, to set matrix size, level count and source names ([Configuring a Jupiter router for CommandCenter](https://wwwapps.grassvalley.com/manuals/commandcenter/core/sub/topic/cc/c_cc_configuring_a_jupiter_router_for_commandcenter.html), search excerpt). The router is configured in a separate tool, then the control layer is set up against it. That is the dead-end ordering Sode wants to avoid.

**Selective import.** The CommandCenter "Router Configuration Backup" page is reported to describe export to a `.ccf` file from the File menu and selective import of components such as Routmaster, Mnemonics, Crosspoints, Locks and Categories ([page](https://wwwapps.grassvalley.com/manuals/commandcenter/core/sub/topic/cc/c_router_configuration_backup.html)). The server returned 403 to direct fetches, so this rests on a search excerpt. Treat it as unverified until someone opens the page in a browser. The component names are close to Sode's own (names, protected destinations, groupings).

**Wizard, access, missing references.** Not found. The NV9000-SE Utilities guide lists import, export and backup of configuration data ([guide](https://www.grassvalley.com/docs/Manuals/routers/nv9000/UG0007-07_NV9000-SE_Utilities_Users_Guide.pdf)), but the direct fetch returned 404, so its contents were not read.

## Sony router control

Sony's Routing Control Station (PWS-110RS1) is described as managing routing for NS-BUS devices, with a Maintenance screen that covers licenses and system backup and restore ([Sony product PDF](https://pro.sony/en_SZ/pdf/products/network-resource-orchestration/pws-110rs1)). Sony's site returned 403, so only a search excerpt was seen, and no setup, access or selective import detail could be confirmed. Third-party manual mirrors turned up in search and were not used.

## Home Assistant

**Steps to a working state.** Three screens. Open the welcome page. Create the owner account (name, lowercase username, password twice) or restore a backup. Finish by setting the home location, optionally data sharing (off by default), and clicking Finish. If compatible devices were found on the network, the last screen lists them ([onboarding](https://www.home-assistant.io/getting-started/onboarding/)).

**What it skips.** Country and time zone are asked only when location detection cannot supply them. Data sharing is opt-in. Integrations are not configured in the wizard. Discovered ones are offered afterward.

**Access at setup.** One decision: the owner account, which is an administrator and can always change all settings. The docs say owner credentials cannot be recovered. No access levels are offered at setup, and more users come later.

**Selective restore.** Backups are full or partial. Parts are Home Assistant settings (the minimum, always included in automatic backups), history, media, share and local app folders, and apps (all, none or selected). On restore you pick the parts. The docs warn the current system is overwritten with the selected parts, restoring settings also restores the version that was running, and a different encryption key prompts for the old key. Restore works from the onboarding screen too, including onto new hardware ([backups](https://www.home-assistant.io/common-tasks/general/backups/), [OS tasks](https://www.home-assistant.io/common-tasks/os/#backups)).

**Missing references.** Not covered by these pages. The docs mention missing hardware only as an instruction to reconnect Zigbee or Z-Wave radios and network storage first.

## Jellyfin

**Steps to a working state.** Six screens: language, administrator account, media libraries, metadata language, networking, finish ([setup wizard](https://jellyfin.org/docs/general/post-install/setup-wizard/)).

**What it skips.** Adding libraries can be skipped by clicking Next, and done later. Other users are created after the wizard.

**Access at setup.** An administrator account is required. The networking screen asks whether to allow remote access, which the docs recommend for most users, and whether to enable automatic port mapping, which the docs recommend leaving off for security. This is the one comparable where an access-reach choice sits inside the wizard.

**Import and missing references.** The wizard page has none. Backup and restore was not read.

## Grafana

**Steps to a working state.** Start the server, sign in at port 3000 with `admin` and `admin`, and change the password when prompted ([sign in](https://grafana.com/docs/grafana/latest/setup-grafana/sign-in-to-grafana/)). There is no wizard. Data sources and dashboards can instead be provisioned from YAML files before first start ([provisioning](https://grafana.com/docs/grafana/latest/administration/provisioning/)).

**What it skips.** Everything. A fresh Grafana has no data source and no dashboard.

**Access at setup.** A known default login plus a prompted password change. Anonymous access was not covered in the page read.

**Provisioning model.** Data sources, plugin apps and dashboards come from files. Order is fixed: `deleteDatasources` runs first, then datasources are added or updated, then dashboards load on startup. `prune: true` removes provisioned data sources no longer in the file. `editable: false` locks a provisioned data source in the UI. Environment variables fill values, not keys or structure ([provisioning](https://grafana.com/docs/grafana/latest/administration/provisioning/)).

**Selective import and missing references.** Importing a dashboard lets you set name, folder and UID, then "Select a data source, if required". Grafana maps the dashboard's data source references to sources that exist in the target ([import dashboards](https://grafana.com/docs/grafana/latest/dashboards/build-dashboards/import-dashboards/)). Export has a "Share dashboard with another instance" option that strips instance-specific details ([share dashboards](https://grafana.com/docs/grafana/latest/dashboards/share-dashboards-panels/)). The docs do not say what happens with a UID conflict or with no matching data source.

## Patterns worth copying

1. Offer restore at first run, as Home Assistant does. A fresh install that offers "restore a backup" next to "create new" gives the travel-show laptop a one-step path from a prior deployment.
2. Make the minimum part mandatory and always included, as Home Assistant does with settings in every backup, so a partial export cannot produce an unusable file.
3. Offer two import modes, as Companion does: merge only the selected categories, or reset everything then import the selection. Say in the button label which one it does.
4. Disable import options the file cannot support, as Companion does, and show the exporter's notes at the top of the import.
5. Add a remap step for references that may not exist on the target, as Companion (connections) and Grafana (data sources) do. For Sode the candidates are destinations, sources and levels that the target router lacks.
6. Strip instance-specific details on export when the file is meant for another site, as Grafana's sharing option does.
7. Ask a reach question in the wizard only when it is real, as Jellyfin does with remote access. For Sode this maps to the access position chosen at setup.
8. Use named access modes, as Ross does: not required, then required, plus a list of trusted addresses. This fits "wide open to full" better than a single on/off.
9. Warn before an access change can lock the user out, as Ross does, and keep access settings out of anything a router reset or an import could silently change.
10. Let a fresh install be configured from files or environment, as Grafana does, so a studio can deploy with no clicks.

## Patterns to avoid

1. No wizard, as in Companion, Ross DashBoard and Grafana. The first working state is a series of unlinked screens, and that was the trigger for this map.
2. An access feature that needs a separate server and license before it works (Ross RBAC through Platform Manager). Sode's access has to work with no extra parts.
3. A known default password (Grafana's `admin`). It needs a forced change and fits a shared laptop badly.
4. Unrecoverable owner credentials with no stated reset path (Home Assistant). A travel-show crew will lose a PIN.
5. Undocumented missing-reference behavior (Companion, Grafana, Ross, Home Assistant). Sode should define it, show it in the import preview and test it.
6. An import that overwrites the current system with no preview. Home Assistant's overwrite warning is its only guard. Show what will change per item first.
7. Category lists the user cannot see. Companion's docs name no categories, and the UI does not say what each button resets.
8. Config split across a configure-first tool and a control tool (Grass Valley Jupiter and CommandCenter). Sode's router link should work from the first IP entry.

## Gaps

- The Ross DashBoard guide PDFs were too large or returned 404 for the fetch tool. The RBAC steps rest on the two help.rossvideo.com pages read in full. The DashBoard guide claims rest on a search excerpt.
- Grass Valley and Sony pages returned 403 or 404. Their selective-import claims are search excerpts and are unverified.
- No vendor page read says how missing references are handled on import. Companion's source code was not read, so its actual behavior for an unmapped connection is unknown.
- Jellyfin backup and restore, and Grafana anonymous access, were not read.
- Companion v5.0 and beta docs were read. The current release is v5.1 and may differ.

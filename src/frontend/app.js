import {
    CheckUpdate,
    ConfirmOrganization,
    GetOrganizationPreview,
    GetMissingDLC,
    GetMissingGames,
    GetMissingUpdates,
    IsKeysFileAvailable,
    LoadSettings,
    OrganizeLibrary,
    RescanLibrary,
    SaveSettings,
    SelectFolder,
    ShowInFolder,
    ShowMessage,
    UpdateDB,
} from './wailsjs/go/app/App.js'
import { EventsOn } from './wailsjs/runtime/runtime.js'

$(function () {

    const THEME_INHERIT = "inherit";
    const THEME_LIGHT = "light";
    const THEME_DARK = "dark";

    let state = {
        settings:{},
        settingsDraft: undefined,
        settingsFeedback: undefined,
        organizationPreview: undefined,
        keys:false
    };

    let currTable
    let currTableExportKind
    let themeMediaQuery
    let themeMediaQueryHandler
    let restartRequired = false

    function tableOptions(options) {
        return Object.assign({
            pagination: "local",
            paginationSize: state.settings.gui.page_size,
            footerElement: '<div class="slm-table-footer-actions"><button type="button" class="btn btn-link export-btn">Export to CSV</button></div>'
        }, options);
    }

    function exportDate() {
        const date = new Date();
        const year = date.getFullYear();
        const month = String(date.getMonth() + 1).padStart(2, "0");
        const day = String(date.getDate()).padStart(2, "0");
        return year + "-" + month + "-" + day;
    }

    function setThemeAttribute(theme) {
        document.documentElement.dataset.bsTheme = theme;
    }

    function applyTheme(theme) {
        if (themeMediaQuery && themeMediaQueryHandler) {
            if (themeMediaQuery.removeEventListener) {
                themeMediaQuery.removeEventListener("change", themeMediaQueryHandler);
            } else if (themeMediaQuery.removeListener) {
                themeMediaQuery.removeListener(themeMediaQueryHandler);
            }
        }
        themeMediaQuery = undefined;
        themeMediaQueryHandler = undefined;

        if (theme !== THEME_LIGHT && theme !== THEME_DARK) {
            if (!window.matchMedia) {
                setThemeAttribute(THEME_LIGHT);
                return;
            }
            themeMediaQuery = window.matchMedia("(prefers-color-scheme: dark)");
            themeMediaQueryHandler = event => setThemeAttribute(event.matches ? THEME_DARK : THEME_LIGHT);
            setThemeAttribute(themeMediaQuery.matches ? THEME_DARK : THEME_LIGHT);
            if (themeMediaQuery.addEventListener) {
                themeMediaQuery.addEventListener("change", themeMediaQueryHandler);
            } else if (themeMediaQuery.addListener) {
                themeMediaQuery.addListener(themeMediaQueryHandler);
            }
            return;
        }

        setThemeAttribute(theme);
    }

    applyTheme(THEME_INHERIT);

    const wrapper = document.querySelector('.wrapper');
    const tabButtons = document.getElementById('tab_btns');
    const tabGroup = document.getElementById('tab-group');
    const progressBar = $('.progress-bar');

    function setLoading(loading) {
        loading = loading || restartRequired;
        wrapper.classList.toggle('is-loading', loading);
        wrapper.setAttribute('aria-busy', String(loading));

        [tabButtons, tabGroup].forEach(element => {
            element.setAttribute('aria-hidden', String(loading));
            if (loading) {
                element.setAttribute('inert', '');
            } else {
                element.removeAttribute('inert');
            }
        });
    }

    function resetProgress() {
        progressBar.attr('aria-valuenow', 0);
        progressBar.attr('style', 'width: 0%');
        progressBar.text('0%');
    }

    function showProgress(message) {
        resetProgress();
        $('.progress-type').text(message);
        setLoading(true);
    }

    function hideProgress() {
        setLoading(false);
    }

    function lockForRestart() {
        restartRequired = true;
        wrapper.classList.add('restart-required');
        $('.progress-type').text('Restart required');
        $('.progress-msg').text('Settings were saved. Close and restart the application to continue.');
        $('.progress-container').attr('aria-label', 'Application restart required');
        setLoading(true);
    }

    //handle tabs action
    $('.tabgroup > div').hide();
    // loadTab($('.tabgroup > div:first-of-type'));

    let showError = function (detail) {
        if (restartRequired) {
            return;
        }
        ShowMessage("error", "Error", "An unexpected error occurred", detail || "")
            .catch(error => console.error(error));
        if (state.settings.paths) {
            state.settings.paths.library_folder = undefined;
        }
        hideProgress();
        loadTab("#library");
    };

        EventsOn("updateProgress", function (message) {
            if (restartRequired) {
                return;
            }
            setLoading(true);
            let count = message.curr;
            let total = message.total;
            $('.progress-msg').text(message.message + " ...");
            if (count >= 0 && total > 0){
                let pcg = Math.min(100, Math.max(0, Math.floor(count / total * 100)));
                $('.progress-bar').attr('aria-valuenow', pcg);
                $('.progress-bar').attr('style', 'width:' + Number(pcg) + '%');
                $('.progress-bar').text(pcg + "%");
            }
        });

        EventsOn("error", showError);
        LoadSettings().then(function (message) {
            state.settings = message;
            state.settingsDraft = undefined;
            state.settingsFeedback = undefined;
            applyTheme(state.settings.gui && state.settings.gui.theme);

            if(state.settings.gui.hide_missing_games){
                document.getElementById("tab_btns").classList.add("hide_missing_games");
            }
        }).catch(error => showError(error.message));

        IsKeysFileAvailable().then(function (message) {
            state.keys = message
        }).catch(error => showError(error.message));

        CheckUpdate().then(function (message) {
            if (!message){
                return
            }
            return ShowMessage("info", "New update available", "There is a new update available, please download from Github", "");
        }).catch(error => showError(error.message));

        showProgress("Downloading latest Switch titles/versions ...");

        UpdateDB().then(function () {
            return scanLocalFolder(false);
        }).catch(error => showError(error.message));

        let openFolderPicker = function (mode) {
            SelectFolder()
                .then(path => updateFolder(mode, path))
                .catch(error => showError(error.message));
        };

        let scanLocalFolder = function(mode){
            if (!state.settings.paths.library_folder){
                loadTab("#library");
                hideProgress();
                return Promise.resolve();
            }
            showProgress("Scanning local library...");

            return RescanLibrary(Boolean(mode)).then(result => {
                state.library = result;
                state.organizationPreview = undefined;
                loadTab("#library");
                hideProgress();
                return result;
            });
        };

        let updateFolder = function (mode,path) {
            if (!path) {
                return
            }

            if (mode === "add"){
                state.settings.paths.scan_folders = state.settings.paths.scan_folders || []
                if (!state.settings.paths.scan_folders.includes(path)){
                    state.settings.paths.scan_folders.push(path);
                }else{
                    return;
                }

            }else{
                state.settings.paths.library_folder = path;
            }
            $('.tabgroup > div').hide();
            showProgress("Scanning local library...");
            console.log("selected folder:"+path);
            state.library = undefined;
            state.updates = undefined;
            state.dlc = undefined;
            state.missingGames = undefined;
            state.organizationPreview = undefined;
            SaveSettings(state.settings)
                .then(() => {
                    state.settingsDraft = undefined;
                    return scanLocalFolder(false);
                })
                .catch(error => showError(error.message));
        };


        function cloneSettings(value) {
            return JSON.parse(JSON.stringify(value));
        }

        function settingsErrorMessage(error) {
            return error && error.message ? error.message : String(error);
        }

        function renderSettingsTab() {
            if (!state.settingsDraft) {
                state.settingsDraft = cloneSettings(state.settings);
            }
            let templateData = Object.assign({}, state.settingsDraft, {
                feedback: state.settingsFeedback
            });
            let settingsHtml = $("#settingsTemplate").render(templateData);
            $("#settings").html(settingsHtml);
        }

        function showSettingsFeedback(type, message) {
            state.settingsFeedback = {type: type, message: message};
            renderSettingsTab();
        }

        function listValues(form, name) {
            return Array.from(form.querySelectorAll('[data-settings-list="' + name + '"]'))
                .map(input => input.value.trim())
                .filter(value => value.length > 0);
        }

        function collectSettings(form) {
            return {
                schema_version: state.settings.schema_version,
                gui: {
                    enabled: form.elements.gui_enabled.checked,
                    page_size: Number(form.elements.gui_page_size.value),
                    hide_missing_games: form.elements.gui_hide_missing_games.checked,
                    hide_demo_games: form.elements.gui_hide_demo_games.checked,
                    theme: form.elements.gui_theme.value
                },
                paths: {
                    library_folder: form.elements.paths_library_folder.value.trim(),
                    scan_folders: listValues(form, "scan_folders"),
                    prod_keys: form.elements.paths_prod_keys.value.trim()
                },
                scan: {
                    recursive: form.elements.scan_recursive.checked,
                    ignore_file_types: listValues(form, "ignore_file_types")
                },
                organization: {
                    create_folder_per_game: form.elements.organization_create_folder_per_game.checked,
                    dlc_folder: form.elements.organization_dlc_folder.value,
                    updates_folder: form.elements.organization_updates_folder.value,
                    rename_files: form.elements.organization_rename_files.checked,
                    delete_empty_folders: form.elements.organization_delete_empty_folders.checked,
                    delete_old_update_files: form.elements.organization_delete_old_update_files.checked,
                    folder_name_template: form.elements.organization_folder_name_template.value,
                    switch_safe_file_names: form.elements.organization_switch_safe_file_names.checked,
                    file_name_template: form.elements.organization_file_name_template.value,
                    process_when_missing_base_game: form.elements.organization_process_when_missing_base_game.checked
                },
                missing_content: {
                    check_for_updates: form.elements.missing_check_for_updates.checked,
                    check_for_dlc: form.elements.missing_check_for_dlc.checked,
                    ignore_dlc_updates: form.elements.missing_ignore_dlc_updates.checked,
                    ignore_dlc_title_ids: listValues(form, "ignore_dlc_title_ids"),
                    ignore_update_title_ids: listValues(form, "ignore_update_title_ids")
                },
                data_sources: {
                    titles_url: form.elements.data_titles_url.value.trim(),
                    versions_url: form.elements.data_versions_url.value.trim()
                },
                logging: {
                    debug: form.elements.logging_debug.checked
                }
            };
        }

        function captureSettingsDraft() {
            const form = document.getElementById("settings-form");
            if (form) {
                state.settingsDraft = collectSettings(form);
            }
        }

        function appendSettingsListRow(name, value) {
            const container = document.querySelector('[data-settings-list-container="' + name + '"]');
            if (!container) {
                return;
            }
            const row = document.createElement("div");
            row.className = "settings-list-row input-group mb-2";
            const input = document.createElement("input");
            input.type = "text";
            input.className = "form-control";
            input.value = value || "";
            input.dataset.settingsList = name;
            const remove = document.createElement("button");
            remove.type = "button";
            remove.className = "btn btn-outline-danger settings-list-remove";
            remove.textContent = "Remove";
            row.append(input, remove);
            container.append(row);
            input.focus();
        }

        function settingsFolderSelected(target, path) {
            if (target === "library_folder") {
                const input = document.querySelector('[name="paths_library_folder"]');
                if (input) {
                    input.value = path;
                }
                captureSettingsDraft();
                return;
            }
            const existing = Array.from(document.querySelectorAll('[data-settings-list="scan_folders"]'))
                .some(input => input.value === path);
            if (!existing) {
                appendSettingsListRow("scan_folders", path);
            }
            captureSettingsDraft();
        }

        function loadTab(target) {
            hideCurrentTab();

            $("#tab_btns a[href='" + target + "']").addClass('active').attr('aria-selected', 'true');
            $(target).show();

            if (target === "#settings") {
                renderSettingsTab();
            } else if (target === "#organize") {
                const folder = state.settings.paths.library_folder;
                if (!folder) {
                    $(target).html($(target + "Template").render({folder: folder}));
                    return;
                }
                if (!state.organizationPreview || state.organizationPreview.root_folder !== folder) {
                    $(target).html($(target + "Template").render({folder: folder, previewLoading: true}));
                    GetOrganizationPreview().then(preview => {
                        state.organizationPreview = preview;
                        if (!restartRequired && $(target).is(":visible")) {
                            loadTab(target);
                        }
                    }).catch(error => showError(error.message));
                    return;
                }
                let html = $(target + "Template").render({folder: folder, preview: state.organizationPreview})
                $(target).html(html);
            } else if (target === "#updates") {
                if (state.settings.paths.library_folder && !state.library){
                    return
                }
                if (state.library && !state.updates){
                    GetMissingUpdates().then(r => {
                        state.updates = r
                        loadTab("#updates")
                    }).catch(error => showError(error.message));
                    return
                }
                let html = $(target + "Template").render({folder: state.settings.paths.library_folder,updates:state.updates})
                $(target).html(html);
                if (state.updates && state.updates.length) {
                    currTable = new Tabulator("#updates-table", tableOptions({
                        layout:"fitDataStretch",
                        initialSort:[
                            {column:"latest_update_date", dir:"desc"}, //sort by this first
                        ],
                        data: state.updates,
                        columns: [
                            {formatter:"rownum",download:false},
                            {field: "Attributes.bannerUrl",download:false,formatter:"image", headerSort:false,formatterParams:{height:"60px", width:"60px"}},
                            {title: "Title", field: "Attributes.name", headerFilter:"input",formatter:"textarea",width:350},
                            {title: "Type", field: "Meta.type", headerFilter:"input"},
                            {title: "Title id", headerSort:false, field: "Attributes.id", hozAlign: "right", sorter: "number"},
                            {title: "Local version", headerSort:false, field: "local_update", hozAlign: "right", sorter: "number"},
                            {title: "Available version", headerSort:false, field: "latest_update", hozAlign: "right"},
                            {title: "Update date", headerSort:true, field: "latest_update_date",sorter:"date", sorterParams:{format:"YYYY-MM-DD"}}
                        ],
                    }));
                    currTableExportKind = "missing_updates";
                }
            } else if (target === "#dlc") {
                if (state.settings.paths.library_folder && !state.library){
                    return
                }
                if (state.library && !state.dlc){
                    GetMissingDLC().then(r => {
                        state.dlc = r
                        loadTab("#dlc")
                    }).catch(error => showError(error.message));
                    return
                }
                let html = $(target + "Template").render({folder: state.settings.paths.library_folder,dlc:state.dlc});
                $(target).html(html);
                if (state.dlc && state.dlc.length) {
                    currTable = new Tabulator("#dlc-table", tableOptions({
                        layout:"fitDataStretch",
                        initialSort:[
                            {column:"Attributes.name", dir:"asc"}, //sort by this first
                        ],
                        data: state.dlc,
                        columns: [
                            {formatter:"rownum",download:false},
                            {field: "Attributes.bannerUrl",download:false,formatter:"image", headerSort:false,formatterParams:{height:"60px", width:"60px"}},
                            {title: "Title", field: "Attributes.name", headerFilter:"input",formatter:"textarea",width:350},
                            {title: "# Missing", field: "missing_dlc.length"},
                            {title: "Missing DLC", headerSort:false, field: "missing_dlc",formatter:function(cell, formatterParams, onRendered){
                                    value = ""
                                    for (var i in cell.getValue())
                                    {
                                        value +="<div>"+cell.getValue()[i]+"</div>"
                                    }
                                    return value
                                }}
                        ],
                    }));
                    currTableExportKind = "missing_dlc";
                }
            } else if (target === "#status") {
                if (state.settings.paths.library_folder && !state.library){
                    return
                }
                let html = $(target + "Template").render({folder: state.settings.paths.library_folder,library:state.library ? state.library.issues: undefined,numFiles:state.library ? state.library.num_files:-1});
                $(target).html(html);
                if (state.library.issues && state.library.issues.length) {
                    currTable = new Tabulator("#status-table", tableOptions({
                        layout:"fitDataStretch",
                        data: state.library.issues,
                        columns: [
                            {formatter:"rownum",download:false},
                            {title: "File name",width:500, headerSort:false, field: "key",formatter:"textarea",cellClick:function(e, cell){
                                    //e - the click event object
                                    //cell - cell component
                                    ShowInFolder(cell.getData().key).catch(error => showError(error.message))
                                }
                            },
                            {
                                title: "Issue", field: "value", width: 350, formatter: function (cell) {
                                    return cell.getValue().replaceAll("\n", "<br/>");
                                }
                            }
                        ],
                    }));
                    currTableExportKind = "issues";
                }
            } else if (target === "#library") {
                if (state.settings.paths.library_folder && !state.library){
                    return
                }
                let html = $(target + "Template").render(
                    {
                        folder: state.settings.paths.library_folder,
                        library: state.library ? state.library.library_data : [] ,
                        num_skipped:state.library ? (state.library.issues ? state.library.issues.length : 0) : 0,
                        num_files:state.library ? state.library.num_files : 0,
                        keys:state.keys
                    })
                $(target).html(html);
                if (state.library && state.library.library_data.length) {
                    currTable = new Tabulator("#library-table", tableOptions({
                        initialSort:[
                            {column:"name", dir:"asc"}, //sort by this first
                        ],
                        layout:"fitColumns",
                        columnMinWidth:24,
                        data: state.library.library_data,
                        columns: [
                            {formatter:"rownum",download:false,minWidth:24},
                            {field: "icon",minWidth:40,formatter:"image", download:false,headerSort:false,formatterParams:{height:"60px", width:"60px"}},
                            {title: "Title", field: "name", headerFilter:"input",formatter:"textarea",widthGrow:3},
                            {title: "Title id", headerSort:false, field: "titleId"},
                            {title: "Region", headerSort:true, field: "region"},
                            {title: "Type", headerSort:true, field: "type"},
                            {title: "Update", headerSort:false, field: "update"},
                            {title: "Version", headerSort:false, field: "version"},
                            {title: "File name", headerSort:false, field: "path",formatter:"textarea",widthGrow:3,cellClick:function(e, cell){
                                    //e - the click event object
                                    //cell - cell component
                                    ShowInFolder(cell.getData().path).catch(error => showError(error.message))
                                }
                            }
                        ],
                    }));
                    currTableExportKind = "games";
                }
            } else if (target === "#missing") {
                if (state.settings.paths.library_folder && !state.library){
                    return
                }
                if (state.library && !state.missingGames){
                    GetMissingGames().then(r => {
                        state.missingGames = r
                        loadTab("#missing")
                    }).catch(error => showError(error.message));
                    return
                }
                let html = $(target + "Template").render({folder: state.settings.paths.library_folder,missingGames:state.missingGames});
                $(target).html(html);
                if (state.missingGames && state.missingGames.length) {
                    currTable = new Tabulator("#missingGames-table", tableOptions({
                        layout:"fitDataStretch",
                        initialSort:[
                            {column:"name", dir:"asc"}, //sort by this first
                        ],
                        data: state.missingGames,
                        columns: [
                            {formatter:"rownum",download:false},
                            {field: "icon",download:false,formatter:"image", headerSort:false,formatterParams:{height:"60px", width:"60px"}},
                            {field: "name",title: "Title",  headerFilter:"input",formatter:"textarea",width:350},
                            {title: "Title id", headerSort:false, field: "titleId"},
                            {title: "Region", headerSort:true,headerFilter:"input",formatter:"textarea", field: "region"},
                            {title: "Release date", headerSort:true, field: "release_date", sorter:"date", sorterParams:{format:"YYYY-MM-DD"}},
                        ],
                    }));
                    currTableExportKind = "missing_games";
                }
            }
        }

        $("body").on("click", ".folder-set", e => {
            openFolderPicker(e.target.textContent)
        });

        $("body").on("click", ".export-btn", e => {
            if (currTable && currTableExportKind) {
                currTable.download("csv", "slm_" + currTableExportKind + "." + exportDate() + ".csv", {}, "all");
            }
        });

        $("body").on("click", ".rescan-action", e => {
            if (restartRequired) {
                return;
            }
            state.library = undefined;
            state.updates = undefined;
            state.dlc = undefined;
            state.missingGames = undefined;
            state.organizationPreview = undefined;
            const hard = e.currentTarget.dataset.hard === "true";
            scanLocalFolder(hard).catch(error => showError(error.message));
        });

        $("body").on("click", ".alert-dismiss", e => {
            $(e.currentTarget).closest(".alert").hide();
        });

        $("body").on("click", "[data-settings-list-add]", e => {
            appendSettingsListRow(e.currentTarget.dataset.settingsListAdd, "");
            captureSettingsDraft();
        });

        $("body").on("click", ".settings-list-remove", e => {
            $(e.currentTarget).closest(".settings-list-row").remove();
            captureSettingsDraft();
        });

        $("body").on("input change", "#settings-form input, #settings-form select", () => {
            captureSettingsDraft();
            state.settingsFeedback = undefined;
        });

        $("body").on("click", ".settings-folder-select", e => {
            SelectFolder()
                .then(path => {
                    if (path) {
                        settingsFolderSelected(e.currentTarget.dataset.settingsFolderTarget, path);
                    }
                })
                .catch(error => showSettingsFeedback("danger", settingsErrorMessage(error)));
        });

        $("body").on("click", "#settings-cancel", () => {
            state.settingsDraft = cloneSettings(state.settings);
            state.settingsFeedback = undefined;
            renderSettingsTab();
        });

        $("body").on("submit", "#settings-form", e => {
            e.preventDefault();
            const value = collectSettings(e.currentTarget);
            state.settingsDraft = value;
            SaveSettings(value)
                .then(() => LoadSettings())
                .then(saved => {
                    state.settings = saved;
                    state.settingsDraft = cloneSettings(saved);
                    state.settingsFeedback = undefined;
                    return ShowMessage(
                        "info",
                        "Restart required",
                        "Settings were saved successfully.",
                        "Close and restart the application for all changes to take effect."
                    ).catch(() => undefined);
                })
                .then(() => lockForRestart())
                .catch(error => showSettingsFeedback("danger", settingsErrorMessage(error)));
        });

        $("body").on("click", ".library-organize-action", e => {
            e.preventDefault();
            if (state.settings.organization.create_folder_per_game === false &&
                state.settings.organization.rename_files === false){
                ShowMessage("info", "Library organization is turned off", "Please update the settings to enable this feature", "Enable 'Rename files' and/or 'Create a folder for each game' in the Settings tab")
                    .catch(error => showError(error.message));
                return
            }
            ConfirmOrganization().then(confirmed => {
                if (confirmed) {
                    //show progress
                    $('.tabgroup > div').hide();
                    showProgress("Organizing local library...");

                    OrganizeLibrary().then(() => {
                        state.library = undefined;
                        state.updates = undefined;
                        state.dlc = undefined;
                        loadTab("#library");
                        return scanLocalFolder(true);
                    }).then(() => {
                        ShowMessage("info", "Success", "Operation completed successfully", "")
                            .catch(error => showError(error.message));
                    }).catch(error => showError(error.message));
                }
            }).catch(error => showError(error.message));

        });

        $('#tab_btns a').click(function (e) {
            e.preventDefault();
            let target = $(e.currentTarget).attr('href');
            loadTab(target);
        });

        function hideCurrentTab() {
            $("#tab_btns a").removeClass("active").attr('aria-selected', 'false');
            let tabgroup = $("#tab_btns").data('tabgroup');
            $("#" + tabgroup).children('div').hide();
        }

});

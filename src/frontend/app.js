import {
    CheckUpdate,
    ConfirmOrganization,
    GetMissingDLC,
    GetMissingGames,
    GetMissingUpdates,
    IsKeysFileAvailable,
    LoadSettings,
    OrganizeLibrary,
    SaveSettings,
    SelectFolder,
    ShowInFolder,
    ShowMessage,
    UpdateDB,
    UpdateLocalLibrary,
} from './wailsjs/go/app/App.js'
import { EventsOn } from './wailsjs/runtime/runtime.js'

$(function () {

    let state = {
        settings:{},
        keys:false
    };

    let currTable

    //handle tabs action
    $('.tabgroup > div').hide();
    // loadTab($('.tabgroup > div:first-of-type'));

    let showError = function (detail) {
        ShowMessage("error", "Error", "An unexpected error occurred", detail || "")
            .catch(error => console.error(error));
        if (state.settings.paths) {
            state.settings.paths.library_folder = undefined;
        }
        $(".progress-container").hide();
        loadTab("#library");
    };

        EventsOn("updateProgress", function (message) {
            let pcg = 0
            let count = message.curr;
            let total = message.total;
            $('.progress-msg').text(message.message + " ...");
            if (count !== -1 && total !== -1){
                pcg = Math.floor(count / total * 100);
                $('.progress-bar').attr('aria-valuenow', pcg);
                $('.progress-bar').attr('style', 'width:' + Number(pcg) + '%');
                $('.progress-bar').text(pcg + "%");
            }
            if (pcg === 100){
                $(".progress-container").hide();
            }else{
                $(".progress-container").show();
            }
        });

        EventsOn("error", showError);
        EventsOn("rescan", function (hard) {
            state.library = undefined;
            state.updates = undefined;
            state.dlc = undefined;
            state.missingGames = undefined;
            scanLocalFolder(Boolean(hard));
        });

        LoadSettings().then(function (message) {
            state.settings = message;

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

        $(".progress-container").show();
        $(".progress-type").text("Downloading latest Switch titles/versions ...");

        UpdateDB().then(function () {
            scanLocalFolder(false);
        }).catch(error => showError(error.message));

        let openFolderPicker = function (mode) {
            SelectFolder()
                .then(path => updateFolder(mode, path))
                .catch(error => showError(error.message));
        };

        let scanLocalFolder = function(mode){
            if (!state.settings.paths.library_folder){
                loadTab("#library")
                return
            }
            //show progress
            $(".progress-container").show();
            $(".progress-type").text("Scanning local library...");

            UpdateLocalLibrary(Boolean(mode))
                .then(result => {
                    state.library = result;
                    loadTab("#library");
                })
                .catch(error => showError(error.message));
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
            console.log("selected folder:"+path);
            state.library = undefined;
            state.updates = undefined;
            state.dlc = undefined;
            SaveSettings(state.settings)
                .then(() => scanLocalFolder(false))
                .catch(error => showError(error.message));
        };


        function loadTab(target) {
            hideCurrentTab();

            $("#tab_btns a[href='" + target + "']").addClass('active');
            $(target).show();

            if (target === "#settings") {
                let settingsJSON = JSON.stringify(state.settings, null, 2)
                let settingsHtml = $(target + "Template").render({code: settingsJSON})
                $(target).html(settingsHtml);
                //  asticode.loader.hide()
            } else if (target === "#organize") {
                let html = $(target + "Template").render({folder: state.settings.paths.library_folder,settings:state.settings})
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
                    currTable = new Tabulator("#updates-table", {
                        layout:"fitDataStretch",
                        initialSort:[
                            {column:"latest_update_date", dir:"desc"}, //sort by this first
                        ],
                        pagination: "local",
                        paginationSize: state.settings.gui.page_size,
                        data: state.updates,
                        columns: [
                            {formatter:"rownum"},
                            {field: "Attributes.bannerUrl",download:false,formatter:"image", headerSort:false,formatterParams:{height:"60px", width:"60px"}},
                            {title: "Title", field: "Attributes.name", headerFilter:"input",formatter:"textarea",width:350},
                            {title: "Type", field: "Meta.type", headerFilter:"input"},
                            {title: "Title id", headerSort:false, field: "Attributes.id", hozAlign: "right", sorter: "number"},
                            {title: "Local version", headerSort:false, field: "local_update", hozAlign: "right", sorter: "number"},
                            {title: "Available version", headerSort:false, field: "latest_update", hozAlign: "right"},
                            {title: "Update date", headerSort:true, field: "latest_update_date",sorter:"date", sorterParams:{format:"YYYY-MM-DD"}}
                        ],
                    });
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
                    currTable = new Tabulator("#dlc-table", {
                        layout:"fitDataStretch",
                        initialSort:[
                            {column:"Attributes.name", dir:"asc"}, //sort by this first
                        ],
                        pagination: "local",
                        paginationSize: state.settings.gui.page_size,
                        data: state.dlc,
                        columns: [
                            {formatter:"rownum"},
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
                    });
                }
            } else if (target === "#status") {
                if (state.settings.paths.library_folder && !state.library){
                    return
                }
                let html = $(target + "Template").render({folder: state.settings.paths.library_folder,library:state.library ? state.library.issues: undefined,numFiles:state.library ? state.library.num_files:-1});
                $(target).html(html);
                if (state.library.issues && state.library.issues.length) {
                    currTable = new Tabulator("#status-table", {
                        layout:"fitDataStretch",
                        pagination: "local",
                        paginationSize: state.settings.gui.page_size,
                        data: state.library.issues,
                        columns: [
                            {formatter:"rownum"},
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
                    });
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
                        keys:state.keys,
                        scanFolders:state.settings.paths.scan_folders
                    })
                $(target).html(html);
                if (state.library && state.library.library_data.length) {
                    currTable = new Tabulator("#library-table", {
                        initialSort:[
                            {column:"name", dir:"asc"}, //sort by this first
                        ],
                        layout:"fitDataStretch",
                        pagination: "local",
                        paginationSize: state.settings.gui.page_size,
                        data: state.library.library_data,
                        columns: [
                            {formatter:"rownum"},
                            {field: "icon",formatter:"image", download:false,headerSort:false,formatterParams:{height:"60px", width:"60px"}},
                            {title: "Title", field: "name", headerFilter:"input",formatter:"textarea",width:350},
                            {title: "Title id", headerSort:false, field: "titleId"},
                            {title: "Region", headerSort:true, field: "region"},
                            {title: "Type", headerSort:true, field: "type"},
                            {title: "Update", headerSort:false, field: "update"},
                            {title: "Version", headerSort:false, field: "version"},
                            {title: "File name", headerSort:false, field: "path",formatter:"textarea",cellClick:function(e, cell){
                                    //e - the click event object
                                    //cell - cell component
                                    ShowInFolder(cell.getData().path).catch(error => showError(error.message))
                                }
                            }
                        ],
                    });
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
                    currTable = new Tabulator("#missingGames-table", {
                        layout:"fitDataStretch",
                        initialSort:[
                            {column:"name", dir:"asc"}, //sort by this first
                        ],
                        pagination: "local",
                        paginationSize: state.settings.gui.page_size,
                        data: state.missingGames,
                        columns: [
                            {formatter:"rownum"},
                            {field: "icon",download:false,formatter:"image", headerSort:false,formatterParams:{height:"60px", width:"60px"}},
                            {field: "name",title: "Title",  headerFilter:"input",formatter:"textarea",width:350},
                            {title: "Title id", headerSort:false, field: "titleId"},
                            {title: "Region", headerSort:true,headerFilter:"input",formatter:"textarea", field: "region"},
                            {title: "Release date", headerSort:true, field: "release_date", sorter:"date", sorterParams:{format:"YYYY-MM-DD"}},
                        ],
                    });
                }
            }
        }

        $("body").on("click", ".folder-set", e => {
            openFolderPicker(e.target.textContent)
        });

        $("body").on("click", ".export-btn", e => {
            currTable.download("csv", "export.csv", {}, "all");
        });

        $("body").on("click", ".library-organize-action", e => {
            e.preventDefault();
            if (state.settings.organization.create_folder_per_game === false &&
                state.settings.organization.rename_files === false){
                ShowMessage("info", "Library organization is turned off", "Please update settings.json to enable this feature", "You should set 'rename_files' and/or 'create_folder_per_game' to 'true'")
                    .catch(error => showError(error.message));
                return
            }
            ConfirmOrganization().then(confirmed => {
                if (confirmed) {
                    //show progress
                    $('.tabgroup > div').hide();
                    $(".progress-container").show();
                    $(".progress-type").text("Organizing local library...");

                    OrganizeLibrary().then(() => {
                        $(".progress-container").hide();
                        state.library = undefined;
                        state.updates = undefined;
                        state.dlc = undefined;
                        loadTab("#library");
                        scanLocalFolder(true);
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
            $("#tab_btns a").removeClass("active");
            let tabgroup = $("#tab_btns").data('tabgroup');
            $("#" + tabgroup).children('div').hide();
        }

});

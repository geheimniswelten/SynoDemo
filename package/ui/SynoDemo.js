/* Synology DSM's application window. */
Ext.namespace("SYNO.SDS.h5uSynoDemo");

Ext.define("SYNO.SDS.h5uSynoDemo.Application", {
    extend: "SYNO.SDS.AppInstance",
    appWindowName: "SYNO.SDS.h5uSynoDemo.MainWindow"
});

Ext.define("SYNO.SDS.h5uSynoDemo.MainWindow", {
    extend: "SYNO.SDS.AppWindow",
    constructor: function (options) {
        this.appInstance = options.appInstance;
        SYNO.SDS.h5uSynoDemo.MainWindow.superclass.constructor.call(this, Ext.apply({
            title: "SynoDemo",
            layout: "fit",
            cls: "syno-app-win",
            width: 1100,
            height: 720,
            minWidth: 640,
            minHeight: 440,
            resizable: true,
            maximizable: true,
            minimizable: true,
            html: '<iframe title="SynoDemo" src="/webman/3rdparty/h5uSynoDemo/index.html" ' +
                'style="display:block;width:100%;height:100%;border:0" ' +
                'referrerpolicy="same-origin"></iframe>'
        }, options));
    }
});

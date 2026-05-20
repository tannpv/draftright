using DraftRightWindows.Services;
using DraftRightWindows.ViewModels;
using WinForms = System.Windows.Forms;

namespace DraftRightWindows.Views;

public sealed class SettingsWindow : IDisposable
{
    public SettingsViewModel ViewModel { get; }

    private WinForms.Form? _form;
    private WinForms.TextBox? _emailBox;
    private WinForms.TextBox? _passwordBox;
    private WinForms.TextBox? _nameBox;
    private WinForms.TextBox? _backendUrlBox;
    private WinForms.Label? _statusLabel;
    private WinForms.Panel? _authPanel;
    private WinForms.Panel? _settingsPanel;
    private WinForms.Button? _submitBtn;
    private WinForms.Button? _eyeBtn;
    private WinForms.CheckBox? _registerCheck;
    private Thread? _formThread;

    public event EventHandler? Closed;

    public SettingsWindow()
    {
        ViewModel = new SettingsViewModel();
    }

    public void Activate()
    {
        if (_form != null && !_form.IsDisposed)
        {
            _form.Invoke(() => { _form.BringToFront(); _form.Activate(); });
            return;
        }

        _formThread = new Thread(ShowForm);
        _formThread.SetApartmentState(ApartmentState.STA);
        _formThread.IsBackground = true;
        _formThread.Start();
    }

    public void Close()
    {
        if (_form != null && !_form.IsDisposed)
            _form.Invoke(() => _form.Close());
    }

    private void ShowForm()
    {
        WinForms.Application.EnableVisualStyles();

        _form = new WinForms.Form
        {
            Text = "DraftRight Settings",
            Width = 420,
            Height = 520,
            StartPosition = WinForms.FormStartPosition.CenterScreen,
            BackColor = System.Drawing.Color.FromArgb(15, 23, 42),
            ForeColor = System.Drawing.Color.FromArgb(226, 232, 240),
            FormBorderStyle = WinForms.FormBorderStyle.FixedSingle,
            MaximizeBox = false,
        };

        var icoPath = System.IO.Path.Combine(
            System.IO.Path.GetDirectoryName(Environment.ProcessPath)!, "Assets", "DraftRight.ico");
        if (System.IO.File.Exists(icoPath))
            _form.Icon = new System.Drawing.Icon(icoPath);

        BuildAuthPanel();
        BuildSettingsPanel();
        UpdateVisibility();

        _form.FormClosed += (_, _) => Closed?.Invoke(this, EventArgs.Empty);
        WinForms.Application.Run(_form);
    }

    private void BuildAuthPanel()
    {
        _authPanel = new WinForms.Panel
        {
            Dock = WinForms.DockStyle.Fill,
            Padding = new WinForms.Padding(24),
        };

        int y = 24;

        // Title
        _authPanel.Controls.Add(MakeLabel("DraftRight Settings", 20, true, ref y));
        y += 12;

        // Register toggle
        _registerCheck = new WinForms.CheckBox
        {
            Text = "Create new account",
            ForeColor = System.Drawing.Color.FromArgb(148, 163, 184),
            Location = new System.Drawing.Point(24, y),
            AutoSize = true,
        };
        _registerCheck.CheckedChanged += (_, _) =>
        {
            ViewModel.IsRegisterMode = _registerCheck.Checked;
            _nameBox!.Visible = _registerCheck.Checked;
            _submitBtn!.Text = _registerCheck.Checked ? "Create Account" : "Sign In";
        };
        _authPanel.Controls.Add(_registerCheck);
        y += 32;

        // Name (hidden by default)
        _authPanel.Controls.Add(MakeLabel("Name", 12, false, ref y));
        _nameBox = MakeTextBox(ref y);
        _nameBox.Visible = false;
        _nameBox.TextChanged += (_, _) => ViewModel.Name = _nameBox.Text;
        _authPanel.Controls.Add(_nameBox);

        // Email
        _authPanel.Controls.Add(MakeLabel("Email", 12, false, ref y));
        _emailBox = MakeTextBox(ref y);
        _emailBox.TextChanged += (_, _) => ViewModel.Email = _emailBox.Text;
        _authPanel.Controls.Add(_emailBox);

        // Password
        _authPanel.Controls.Add(MakeLabel("Password", 12, false, ref y));
        _passwordBox = MakeTextBox(ref y);
        _passwordBox.Size = new System.Drawing.Size(310, 30);
        _passwordBox.UseSystemPasswordChar = true;
        _passwordBox.TextChanged += (_, _) => ViewModel.Password = _passwordBox.Text;
        _authPanel.Controls.Add(_passwordBox);

        // Password visibility toggle
        _eyeBtn = new WinForms.Button
        {
            Text = "\uD83D\uDC41",
            Location = new System.Drawing.Point(_passwordBox.Location.X + 314, _passwordBox.Location.Y),
            Size = new System.Drawing.Size(30, 30),
            FlatStyle = WinForms.FlatStyle.Flat,
            BackColor = System.Drawing.Color.FromArgb(30, 41, 59),
            ForeColor = System.Drawing.Color.FromArgb(148, 163, 184),
            Font = new System.Drawing.Font("Segoe UI", 9),
            Cursor = WinForms.Cursors.Hand,
        };
        _eyeBtn.FlatAppearance.BorderSize = 0;
        _eyeBtn.Click += (_, _) =>
        {
            _passwordBox.UseSystemPasswordChar = !_passwordBox.UseSystemPasswordChar;
            _eyeBtn.Text = _passwordBox.UseSystemPasswordChar ? "\uD83D\uDC41" : "\uD83D\uDD12";
        };
        _authPanel.Controls.Add(_eyeBtn);

        // Submit
        _submitBtn = new WinForms.Button
        {
            Text = "Sign In",
            Location = new System.Drawing.Point(24, y),
            Size = new System.Drawing.Size(350, 38),
            BackColor = System.Drawing.Color.FromArgb(93, 135, 255),
            ForeColor = System.Drawing.Color.White,
            FlatStyle = WinForms.FlatStyle.Flat,
            Font = new System.Drawing.Font("Segoe UI", 10, System.Drawing.FontStyle.Bold),
        };
        _submitBtn.FlatAppearance.BorderSize = 0;
        _submitBtn.Click += OnAuthSubmit;
        _authPanel.Controls.Add(_submitBtn);
        y += 48;

        // Status
        _statusLabel = new WinForms.Label
        {
            Location = new System.Drawing.Point(24, y),
            Size = new System.Drawing.Size(350, 40),
            ForeColor = System.Drawing.Color.FromArgb(239, 68, 68),
            Font = new System.Drawing.Font("Segoe UI", 9),
        };
        _authPanel.Controls.Add(_statusLabel);

        _form!.Controls.Add(_authPanel);
    }

    private void BuildSettingsPanel()
    {
        _settingsPanel = new WinForms.Panel
        {
            Dock = WinForms.DockStyle.Fill,
            Padding = new WinForms.Padding(24),
            Visible = false,
        };

        int y = 24;
        _settingsPanel.Controls.Add(MakeLabel("DraftRight Settings", 20, true, ref y));
        y += 12;

        // Signed in as
        var emailLabel = new WinForms.Label
        {
            Text = $"Signed in as: {ViewModel.LoggedInEmail ?? ""}",
            Location = new System.Drawing.Point(24, y),
            AutoSize = true,
            ForeColor = System.Drawing.Color.FromArgb(148, 163, 184),
            Font = new System.Drawing.Font("Segoe UI", 10),
        };
        _settingsPanel.Controls.Add(emailLabel);
        y += 32;

        // Backend URL
        _settingsPanel.Controls.Add(MakeLabel("Backend URL", 12, false, ref y));
        _backendUrlBox = MakeTextBox(ref y);
        _backendUrlBox.Text = ViewModel.BackendUrl ?? "";
        _backendUrlBox.TextChanged += (_, _) => ViewModel.BackendUrl = _backendUrlBox.Text;
        _settingsPanel.Controls.Add(_backendUrlBox);

        // Save
        var saveBtn = new WinForms.Button
        {
            Text = "Save Settings",
            Location = new System.Drawing.Point(24, y),
            Size = new System.Drawing.Size(350, 38),
            BackColor = System.Drawing.Color.FromArgb(93, 135, 255),
            ForeColor = System.Drawing.Color.White,
            FlatStyle = WinForms.FlatStyle.Flat,
            Font = new System.Drawing.Font("Segoe UI", 10, System.Drawing.FontStyle.Bold),
        };
        saveBtn.FlatAppearance.BorderSize = 0;
        saveBtn.Click += (_, _) =>
        {
            if (ViewModel.SaveSettingsCommand.CanExecute(null))
                ViewModel.SaveSettingsCommand.Execute(null);
        };
        _settingsPanel.Controls.Add(saveBtn);
        y += 48;

        // Sign out
        var signOutBtn = new WinForms.Button
        {
            Text = "Sign Out",
            Location = new System.Drawing.Point(24, y),
            Size = new System.Drawing.Size(350, 38),
            BackColor = System.Drawing.Color.Transparent,
            ForeColor = System.Drawing.Color.FromArgb(226, 232, 240),
            FlatStyle = WinForms.FlatStyle.Flat,
            Font = new System.Drawing.Font("Segoe UI", 10),
        };
        signOutBtn.FlatAppearance.BorderColor = System.Drawing.Color.FromArgb(51, 65, 85);
        signOutBtn.Click += (_, _) =>
        {
            if (ViewModel.LogoutCommand.CanExecute(null))
                ViewModel.LogoutCommand.Execute(null);
            UpdateVisibility();
        };
        _settingsPanel.Controls.Add(signOutBtn);

        _form!.Controls.Add(_settingsPanel);
    }

    private void UpdateVisibility()
    {
        if (_authPanel == null || _settingsPanel == null) return;
        _authPanel.Visible = !ViewModel.IsLoggedIn;
        _settingsPanel.Visible = ViewModel.IsLoggedIn;
    }

    private async void OnAuthSubmit(object? sender, EventArgs e)
    {
        _statusLabel!.Text = "";
        _submitBtn!.Enabled = false;
        DRLogger.Log($"Sign-in attempt: email={_emailBox!.Text}", DRLogger.Category.AUTH);

        try
        {
            if (ViewModel.IsRegisterMode)
                await ViewModel.RegisterCommand.ExecuteAsync(null);
            else
                await ViewModel.LoginCommand.ExecuteAsync(null);

            if (!string.IsNullOrEmpty(ViewModel.ErrorMessage))
            {
                _statusLabel.Text = ViewModel.ErrorMessage;
                DRLogger.Error($"Sign-in FAILED: {ViewModel.ErrorMessage}", DRLogger.Category.AUTH);
            }
            else
            {
                DRLogger.Log("Sign-in SUCCESS", DRLogger.Category.AUTH);
                UpdateVisibility();
            }
        }
        catch (Exception ex)
        {
            _statusLabel.Text = ex.Message;
            DRLogger.Error($"Sign-in FAILED: {ex.Message}", DRLogger.Category.AUTH);
        }
        finally
        {
            _submitBtn.Enabled = true;
        }
    }

    // ── Helpers ──

    private static WinForms.Label MakeLabel(string text, int fontSize, bool bold, ref int y)
    {
        var lbl = new WinForms.Label
        {
            Text = text,
            Location = new System.Drawing.Point(24, y),
            AutoSize = true,
            ForeColor = bold
                ? System.Drawing.Color.FromArgb(226, 232, 240)
                : System.Drawing.Color.FromArgb(148, 163, 184),
            Font = new System.Drawing.Font("Segoe UI", fontSize,
                bold ? System.Drawing.FontStyle.Bold : System.Drawing.FontStyle.Regular),
        };
        y += bold ? (fontSize + 16) : (fontSize + 10);
        return lbl;
    }

    private static WinForms.TextBox MakeTextBox(ref int y)
    {
        var tb = new WinForms.TextBox
        {
            Location = new System.Drawing.Point(24, y),
            Size = new System.Drawing.Size(350, 30),
            BackColor = System.Drawing.Color.FromArgb(30, 41, 59),
            ForeColor = System.Drawing.Color.FromArgb(226, 232, 240),
            BorderStyle = WinForms.BorderStyle.FixedSingle,
            Font = new System.Drawing.Font("Segoe UI", 10),
        };
        y += 38;
        return tb;
    }

    public void Dispose()
    {
        Close();
    }
}

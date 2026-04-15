using System;
using System.Collections.Generic;
using System.Threading.Tasks;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;

namespace DraftRightWindows.ViewModels;

public partial class SettingsViewModel : ObservableObject
{
    // ── Auth fields ──

    [ObservableProperty]
    private string _email = string.Empty;

    [ObservableProperty]
    private string _password = string.Empty;

    [ObservableProperty]
    private string _name = string.Empty;

    [ObservableProperty]
    private bool _isLoggedIn;

    [ObservableProperty]
    private bool _isLoading;

    [ObservableProperty]
    private string _errorMessage = string.Empty;

    [ObservableProperty]
    private bool _isRegisterMode;

    // ── Settings fields ──

    [ObservableProperty]
    private string _backendUrl = string.Empty;

    [ObservableProperty]
    private string _translateLanguage = string.Empty;

    [ObservableProperty]
    private bool _autoStart;

    [ObservableProperty]
    private string _loggedInEmail = string.Empty;

    // ── Static data ──

    public static IReadOnlyList<string> SupportedLanguages { get; } = new[]
    {
        "Arabic",
        "Bengali",
        "Chinese (Simplified)",
        "Chinese (Traditional)",
        "Czech",
        "Danish",
        "Dutch",
        "English",
        "Finnish",
        "French",
        "German",
        "Greek",
        "Hebrew",
        "Hindi",
        "Hungarian",
        "Indonesian",
        "Italian",
        "Japanese",
        "Korean",
        "Malay",
        "Norwegian",
        "Polish",
        "Portuguese",
        "Romanian",
        "Russian",
        "Spanish",
        "Swedish",
        "Thai",
        "Turkish",
        "Vietnamese"
    };

    public string HotkeyDisplay => "Ctrl+Shift+R";

    // ── Constructor ──

    public SettingsViewModel()
    {
        LoadCurrentSettings();
    }

    private void LoadCurrentSettings()
    {
        var settings = App.Settings;
        BackendUrl = settings.BackendUrl;
        TranslateLanguage = settings.TranslateLanguage;
        AutoStart = settings.AutoStart;

        IsLoggedIn = App.Auth.IsLoggedIn;
        if (IsLoggedIn)
        {
            LoggedInEmail = App.Auth.CurrentEmail;
        }
    }

    // ── Commands ──

    [RelayCommand]
    private async Task LoginAsync()
    {
        if (string.IsNullOrWhiteSpace(Email) || string.IsNullOrWhiteSpace(Password))
        {
            ErrorMessage = "Email and password are required.";
            return;
        }

        ErrorMessage = string.Empty;
        IsLoading = true;

        try
        {
            var result = await App.Api.LoginAsync(Email, Password);
            App.Auth.SaveTokens(result.AccessToken, result.RefreshToken, result.User.Email);
            App.Api.SetToken(result.AccessToken);

            IsLoggedIn = true;
            LoggedInEmail = result.User.Email;
            Password = string.Empty;
        }
        catch (Services.ApiException ex)
        {
            ErrorMessage = ex.Message;
        }
        catch (Exception ex)
        {
            ErrorMessage = $"Login failed: {ex.Message}";
        }
        finally
        {
            IsLoading = false;
        }
    }

    [RelayCommand]
    private async Task RegisterAsync()
    {
        if (string.IsNullOrWhiteSpace(Email) || string.IsNullOrWhiteSpace(Password))
        {
            ErrorMessage = "Email and password are required.";
            return;
        }

        if (string.IsNullOrWhiteSpace(Name))
        {
            ErrorMessage = "Name is required for registration.";
            return;
        }

        ErrorMessage = string.Empty;
        IsLoading = true;

        try
        {
            var result = await App.Api.RegisterAsync(Email, Password, Name);
            App.Auth.SaveTokens(result.AccessToken, result.RefreshToken, result.User.Email);
            App.Api.SetToken(result.AccessToken);

            IsLoggedIn = true;
            LoggedInEmail = result.User.Email;
            Password = string.Empty;
        }
        catch (Services.ApiException ex)
        {
            ErrorMessage = ex.Message;
        }
        catch (Exception ex)
        {
            ErrorMessage = $"Registration failed: {ex.Message}";
        }
        finally
        {
            IsLoading = false;
        }
    }

    [RelayCommand]
    private void Logout()
    {
        App.Auth.ClearTokens();
        App.Api.ClearToken();
        IsLoggedIn = false;
        LoggedInEmail = string.Empty;
        Email = string.Empty;
        Password = string.Empty;
        Name = string.Empty;
    }

    [RelayCommand]
    private void SaveSettings()
    {
        var settings = App.Settings;
        settings.BackendUrl = BackendUrl;
        settings.TranslateLanguage = TranslateLanguage;
        settings.AutoStart = AutoStart;
        settings.Save();

        ErrorMessage = string.Empty;
    }

    /// <summary>Toggle between Login and Register modes.</summary>
    [RelayCommand]
    private void ToggleAuthMode()
    {
        IsRegisterMode = !IsRegisterMode;
        ErrorMessage = string.Empty;
    }
}

#ifndef WIN32_LEAN_AND_MEAN
    #define WIN32_LEAN_AND_MEAN 1
#endif

#undef GAME_INTEGRATION_EXPORT

#include <windows.h>

#include "tobii_gameintegration_dynamic_loader.h"
#include "tobii_gameintegration.h"

namespace TobiiGameIntegration
{

#ifdef _WIN64
#ifdef _DEBUG
    const char g_DllName[] = "tobii_gameintegration_x64_d.dll";
#else
    const char g_DllName[] = "tobii_gameintegration_x64.dll";
#endif
#else
#ifdef _DEBUG
    const char g_DllName[] = "tobii_gameintegration_x86_d.dll";
#else
    const char g_DllName[] = "tobii_gameintegration_x86.dll";
#endif
#endif

    static GameIntegrationLoader g_GameIntegrationLoader;

    ITobiiGameIntegrationApi* __cdecl GetApiDynamic(const char* titleName, const char* dllPath, int majorVersion, int minorVersion, int revision, const uint16_t* license, uint32_t licenseSize, bool analyticalUse)
    {
        return g_GameIntegrationLoader.LinkAndGetApi(titleName, dllPath, majorVersion, minorVersion, revision, license, licenseSize, analyticalUse);
    }

    bool GameIntegrationLoader::Load(const char* dllPath)
    {
        if (!IsLoaded())
        {
            HINSTANCE hinst;
            if (dllPath == nullptr)
            {
                hinst = LoadLibraryA(g_DllName);
            }
            else
            {
                hinst = LoadLibraryA(dllPath);
            }

            if (hinst != 0)
            {
                m_hInstance = reinterpret_cast<void*>(hinst);
            }
            else
            {
                m_LastError = GetLastError();
            }
        }
        return(IsLoaded());
    }

    bool GameIntegrationLoader::Link()
    {
        if (!IsLinked() && IsLoaded())
        {
            HINSTANCE hinst = reinterpret_cast<HINSTANCE>(m_hInstance);
            const char* functionName = "GetApi";
            m_GetApiFunctionPointer = reinterpret_cast<GetApiFunction>(GetProcAddress(hinst, functionName));
            if (m_GetApiFunctionPointer != 0)
            {
                m_Linked = true;
            }
            else
            {
                Unload();
            }
        }
        return(IsLinked());
    }

    void GameIntegrationLoader::Unload()
    {
        if (m_hInstance != 0)
        {
            m_Linked = false;
            FreeLibrary(reinterpret_cast<HINSTANCE>(m_hInstance));
            m_hInstance = 0;
        }
    }

    ITobiiGameIntegrationApi* GameIntegrationLoader::LinkAndGetApi(
        const char* titleName,
        const char* dllPath,
        int majorVersion,
        int minorVersion,
        int revision,
        const uint16_t* license,
        uint32_t licenseSize,
        bool analyticalUse
    )
    {
        if (g_GameIntegrationLoader.Load(dllPath) && g_GameIntegrationLoader.Link())
        {
            return g_GameIntegrationLoader.m_GetApiFunctionPointer(titleName, majorVersion, minorVersion, revision, license, licenseSize, analyticalUse);
        }

        return nullptr;
    }

    DWORD GameIntegrationLoader::GetLastErrorFromWindows()
    {
        return m_LastError;
    }
}
